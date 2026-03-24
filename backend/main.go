package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
    "context"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "github.com/lib/pq"
    "github.com/redis/go-redis/v9"
    "golang.org/x/crypto/bcrypt"
    "github.com/golang-jwt/jwt/v5"
)

var (
    upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
    }
    db           *sql.DB
    redisClient  *redis.Client
    clients      = make(map[*websocket.Conn]string)
    broadcast    = make(chan Message)
)

type User struct {
    ID        int    `json:"id"`
    Phone     string `json:"phone"`
    Name      string `json:"name"`
    Password  string `json:"password,omitempty"`
    CreatedAt string `json:"created_at"`
}

type Message struct {
    ID        int    `json:"id"`
    FromUser  int    `json:"from_user"`
    ToUser    int    `json:"to_user"`
    Content   string `json:"content"`
    Type      string `json:"type"`
    Timestamp string `json:"timestamp"`
    Status    string `json:"status"`
}

type LoginRequest struct {
    Phone    string `json:"phone"`
    Password string `json:"password"`
}

type RegisterRequest struct {
    Phone    string `json:"phone"`
    Name     string `json:"name"`
    Password string `json:"password"`
}

func initDB() {
    var err error
    connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"),
        os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
    
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    // Test connection
    err = db.Ping()
    if err != nil {
        log.Fatal("Failed to ping database:", err)
    }

    // Create tables
    createTableSQL := `
    CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        phone VARCHAR(15) UNIQUE NOT NULL,
        name VARCHAR(100) NOT NULL,
        password VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE TABLE IF NOT EXISTS messages (
        id SERIAL PRIMARY KEY,
        from_user INTEGER REFERENCES users(id) ON DELETE CASCADE,
        to_user INTEGER REFERENCES users(id) ON DELETE CASCADE,
        content TEXT NOT NULL,
        type VARCHAR(20) DEFAULT 'text',
        timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        status VARCHAR(20) DEFAULT 'sent'
    );
    
    CREATE INDEX IF NOT EXISTS idx_messages_users ON messages(from_user, to_user);
    CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);`
    
    _, err = db.Exec(createTableSQL)
    if err != nil {
        log.Fatal("Failed to create tables:", err)
    }
    
    log.Println("Database initialized successfully")
}

func initRedis() {
    redisAddr := fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))
    redisClient = redis.NewClient(&redis.Options{
        Addr:     redisAddr,
        Password: "",
        DB:       0,
    })
    
    ctx := context.Background()
    _, err := redisClient.Ping(ctx).Result()
    if err != nil {
        log.Fatal("Failed to connect to Redis:", err)
    }
    log.Println("Redis connected successfully")
}

func registerUser(c *gin.Context) {
    var req RegisterRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    // Validate input
    if req.Phone == "" || req.Name == "" || req.Password == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
        return
    }
    
    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }
    
    var userID int
    err = db.QueryRow(
        "INSERT INTO users (phone, name, password) VALUES ($1, $2, $3) RETURNING id",
        req.Phone, req.Name, string(hashedPassword),
    ).Scan(&userID)
    
    if err != nil {
        if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
            c.JSON(http.StatusConflict, gin.H{"error": "Phone number already registered"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }
    
    token := generateJWT(userID)
    c.JSON(http.StatusOK, gin.H{
        "user_id": userID,
        "token":   token,
        "message": "User registered successfully",
    })
}

func loginUser(c *gin.Context) {
    var req LoginRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    var user User
    var hashedPassword string
    err := db.QueryRow(
        "SELECT id, phone, name, password FROM users WHERE phone = $1",
        req.Phone,
    ).Scan(&user.ID, &user.Phone, &user.Name, &hashedPassword)
    
    if err != nil {
        if err == sql.ErrNoRows {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    
    err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }
    
    token := generateJWT(user.ID)
    c.JSON(http.StatusOK, gin.H{
        "user_id": user.ID,
        "name":    user.Name,
        "phone":   user.Phone,
        "token":   token,
    })
}

func generateJWT(userID int) string {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
        "iat":     time.Now().Unix(),
    })
    
    tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
    return tokenString
}

func authMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := c.GetHeader("Authorization")
        if tokenString == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "No authorization token"})
            c.Abort()
            return
        }
        
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(os.Getenv("JWT_SECRET")), nil
        })
        
        if err != nil || !token.Valid {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }
        
        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
            c.Abort()
            return
        }
        
        userID, ok := claims["user_id"].(float64)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
            c.Abort()
            return
        }
        
        c.Set("user_id", int(userID))
        c.Next()
    }
}

func handleWebSocket(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        log.Println("Failed to upgrade connection:", err)
        return
    }
    defer conn.Close()
    
    userID := c.GetInt("user_id")
    clients[conn] = fmt.Sprintf("%d", userID)
    
    // Store user connection in Redis
    ctx := context.Background()
    redisClient.Set(ctx, fmt.Sprintf("user:%d", userID), "online", time.Hour*24)
    
    // Load offline messages
    loadOfflineMessages(conn, userID)
    
    for {
        var msg Message
        err := conn.ReadJSON(&msg)
        if err != nil {
            log.Printf("User %d disconnected: %v", userID, err)
            delete(clients, conn)
            redisClient.Del(ctx, fmt.Sprintf("user:%d", userID))
            break
        }
        
        msg.FromUser = userID
        msg.Timestamp = time.Now().Format(time.RFC3339)
        
        // Save to database
        var msgID int
        err = db.QueryRow(
            "INSERT INTO messages (from_user, to_user, content, type, status) VALUES ($1, $2, $3, $4, 'sent') RETURNING id",
            msg.FromUser, msg.ToUser, msg.Content, msg.Type,
        ).Scan(&msgID)
        
        if err != nil {
            log.Println("Failed to save message:", err)
            continue
        }
        msg.ID = msgID
        
        // Try to deliver immediately
        delivered := false
        for client, id := range clients {
            if id == fmt.Sprintf("%d", msg.ToUser) {
                err := client.WriteJSON(msg)
                if err == nil {
                    delivered = true
                    // Update message status
                    db.Exec("UPDATE messages SET status = 'delivered' WHERE id = $1", msgID)
                }
                break
            }
        }
        
        // Store in Redis if user is offline
        if !delivered {
            storeOfflineMessage(msg.ToUser, msg)
        }
    }
}

func storeOfflineMessage(userID int, msg Message) {
    ctx := context.Background()
    key := fmt.Sprintf("offline:%d", userID)
    data, _ := json.Marshal(msg)
    redisClient.LPush(ctx, key, data)
    redisClient.Expire(ctx, key, time.Hour*24*7) // Keep offline messages for 7 days
}

func loadOfflineMessages(conn *websocket.Conn, userID int) {
    ctx := context.Background()
    key := fmt.Sprintf("offline:%d", userID)
    
    messages, err := redisClient.LRange(ctx, key, 0, -1).Result()
    if err != nil {
        return
    }
    
    for i := len(messages) - 1; i >= 0; i-- {
        var msg Message
        if err := json.Unmarshal([]byte(messages[i]), &msg); err != nil {
            continue
        }
        if err := conn.WriteJSON(msg); err != nil {
            log.Printf("Failed to send offline message to user %d: %v", userID, err)
        } else {
            // Update message status
            db.Exec("UPDATE messages SET status = 'delivered' WHERE id = $1", msg.ID)
        }
    }
    
    redisClient.Del(ctx, key)
}

func getUsers(c *gin.Context) {
    userID := c.GetInt("user_id")
    
    rows, err := db.Query("SELECT id, phone, name FROM users WHERE id != $1 ORDER BY name", userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
        return
    }
    defer rows.Close()
    
    var users []User
    ctx := context.Background()
    
    for rows.Next() {
        var u User
        rows.Scan(&u.ID, &u.Phone, &u.Name)
        
        // Check online status
        _, err := redisClient.Get(ctx, fmt.Sprintf("user:%d", u.ID)).Result()
        if err == nil {
            u.Name = u.Name + " (Online)"
        }
        
        users = append(users, u)
    }
    
    c.JSON(http.StatusOK, users)
}

func getMessages(c *gin.Context) {
    userID := c.GetInt("user_id")
    otherUserID := c.Param("userId")
    
    rows, err := db.Query(`
        SELECT id, from_user, to_user, content, type, timestamp, status 
        FROM messages 
        WHERE (from_user = $1 AND to_user = $2) OR (from_user = $2 AND to_user = $1)
        ORDER BY timestamp ASC LIMIT 100`,
        userID, otherUserID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
        return
    }
    defer rows.Close()
    
    var messages []Message
    for rows.Next() {
        var m Message
        rows.Scan(&m.ID, &m.FromUser, &m.ToUser, &m.Content, &m.Type, &m.Timestamp, &m.Status)
        messages = append(messages, m)
    }
    
    c.JSON(http.StatusOK, messages)
}

func main() {
    // Initialize database and Redis
    initDB()
    initRedis()
    
    // Set Gin mode
    if os.Getenv("GIN_MODE") == "release" {
        gin.SetMode(gin.ReleaseMode)
    }
    
    router := gin.Default()
    
    // CORS middleware
    router.Use(func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        c.Writer.Header().Set("Access-Control-Max-Age", "86400")
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    })
    
    // Public routes
    router.POST("/api/register", registerUser)
    router.POST("/api/login", loginUser)
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "healthy"})
    })
    
    // Protected routes
    authorized := router.Group("/api")
    authorized.Use(authMiddleware())
    {
        authorized.GET("/users", getUsers)
        authorized.GET("/messages/:userId", getMessages)
        authorized.GET("/ws", handleWebSocket)
    }
    
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    
    log.Printf("Server starting on port %s", port)
    router.Run(":" + port)
}

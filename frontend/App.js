import React, { useState, useEffect, useRef } from 'react';
import {
  SafeAreaView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
  FlatList,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { NavigationContainer } from '@react-navigation/native';
import { createStackNavigator } from '@react-navigation/stack';
import { GiftedChat, Bubble, Send } from 'react-native-gifted-chat';
import Icon from 'react-native-vector-icons/MaterialIcons';

const Stack = createStackNavigator();
const API_URL = 'http://your-backend-service/api'; // Update with your K8s service URL
const WS_URL = 'ws://your-backend-service/api/ws'; // Update with your K8s service URL

// Login Screen
function LoginScreen({ navigation }) {
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async () => {
    if (!phone || !password) {
      Alert.alert('Error', 'Please fill all fields');
      return;
    }

    setLoading(true);
    try {
      const response = await fetch(`${API_URL}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone, password }),
      });

      const data = await response.json();
      if (response.ok) {
        await AsyncStorage.setItem('userToken', data.token);
        await AsyncStorage.setItem('userId', data.user_id.toString());
        await AsyncStorage.setItem('userName', data.name);
        navigation.replace('ChatList');
      } else {
        Alert.alert('Error', data.error || 'Login failed');
      }
    } catch (error) {
      Alert.alert('Error', 'Network error');
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = () => {
    navigation.navigate('Register');
  };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>WhatsApp Clone</Text>
        <Text style={styles.subtitle}>Login to continue</Text>
      </View>
      <View style={styles.form}>
        <TextInput
          style={styles.input}
          placeholder="Phone Number"
          value={phone}
          onChangeText={setPhone}
          keyboardType="phone-pad"
        />
        <TextInput
          style={styles.input}
          placeholder="Password"
          value={password}
          onChangeText={setPassword}
          secureTextEntry
        />
        <TouchableOpacity style={styles.button} onPress={handleLogin} disabled={loading}>
          <Text style={styles.buttonText}>{loading ? 'Logging in...' : 'Login'}</Text>
        </TouchableOpacity>
        <TouchableOpacity onPress={handleRegister}>
          <Text style={styles.linkText}>Don't have an account? Register</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

// Register Screen
function RegisterScreen({ navigation }) {
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const handleRegister = async () => {
    if (!name || !phone || !password) {
      Alert.alert('Error', 'Please fill all fields');
      return;
    }

    setLoading(true);
    try {
      const response = await fetch(`${API_URL}/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, phone, password }),
      });

      const data = await response.json();
      if (response.ok) {
        await AsyncStorage.setItem('userToken', data.token);
        await AsyncStorage.setItem('userId', data.user_id.toString());
        await AsyncStorage.setItem('userName', name);
        navigation.replace('ChatList');
      } else {
        Alert.alert('Error', data.error || 'Registration failed');
      }
    } catch (error) {
      Alert.alert('Error', 'Network error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Create Account</Text>
      </View>
      <View style={styles.form}>
        <TextInput
          style={styles.input}
          placeholder="Name"
          value={name}
          onChangeText={setName}
        />
        <TextInput
          style={styles.input}
          placeholder="Phone Number"
          value={phone}
          onChangeText={setPhone}
          keyboardType="phone-pad"
        />
        <TextInput
          style={styles.input}
          placeholder="Password"
          value={password}
          onChangeText={setPassword}
          secureTextEntry
        />
        <TouchableOpacity style={styles.button} onPress={handleRegister} disabled={loading}>
          <Text style={styles.buttonText}>{loading ? 'Registering...' : 'Register'}</Text>
        </TouchableOpacity>
        <TouchableOpacity onPress={() => navigation.goBack()}>
          <Text style={styles.linkText}>Already have an account? Login</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

// Chat List Screen
function ChatListScreen({ navigation }) {
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchUsers();
    const interval = setInterval(fetchUsers, 5000); // Refresh every 5 seconds
    return () => clearInterval(interval);
  }, []);

  const fetchUsers = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      const response = await fetch(`${API_URL}/users`, {
        headers: { 'Authorization': token },
      });
      const data = await response.json();
      if (response.ok) {
        setUsers(data);
      }
    } catch (error) {
      console.error('Error fetching users:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = async () => {
    await AsyncStorage.clear();
    navigation.replace('Login');
  };

  const renderUser = ({ item }) => (
    <TouchableOpacity
      style={styles.userItem}
      onPress={() => navigation.navigate('Chat', { userId: item.id, userName: item.name })}
    >
      <View style={styles.userAvatar}>
        <Icon name="person" size={30} color="#fff" />
      </View>
      <View style={styles.userInfo}>
        <Text style={styles.userName}>{item.name}</Text>
        <Text style={styles.userPhone}>{item.phone}</Text>
      </View>
    </TouchableOpacity>
  );

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.chatListHeader}>
        <Text style={styles.headerTitle}>Chats</Text>
        <TouchableOpacity onPress={handleLogout}>
          <Icon name="logout" size={24} color="#075E54" />
        </TouchableOpacity>
      </View>
      {loading ? (
        <Text style={styles.loadingText}>Loading...</Text>
      ) : (
        <FlatList
          data={users}
          renderItem={renderUser}
          keyExtractor={(item) => item.id.toString()}
          ListEmptyComponent={<Text style={styles.emptyText}>No users found</Text>}
        />
      )}
    </SafeAreaView>
  );
}

// Chat Screen
function ChatScreen({ route, navigation }) {
  const { userId, userName } = route.params;
  const [messages, setMessages] = useState([]);
  const [ws, setWs] = useState(null);
  const [currentUserId, setCurrentUserId] = useState(null);

  useEffect(() => {
    loadCurrentUser();
    loadMessages();
    connectWebSocket();

    return () => {
      if (ws) {
        ws.close();
      }
    };
  }, []);

  const loadCurrentUser = async () => {
    const userId = await AsyncStorage.getItem('userId');
    setCurrentUserId(parseInt(userId));
  };

  const loadMessages = async () => {
    try {
      const token = await AsyncStorage.getItem('userToken');
      const response = await fetch(`${API_URL}/messages/${userId}`, {
        headers: { 'Authorization': token },
      });
      const data = await response.json();
      if (response.ok) {
        const formattedMessages = data.map(msg => ({
          _id: msg.id,
          text: msg.content,
          createdAt: new Date(msg.timestamp),
          user: {
            _id: msg.from_user,
          },
        }));
        setMessages(formattedMessages.reverse());
      }
    } catch (error) {
      console.error('Error loading messages:', error);
    }
  };

  const connectWebSocket = async () => {
    const token = await AsyncStorage.getItem('userToken');
    const wsUrl = `${WS_URL}?token=${token}`;
    const websocket = new WebSocket(wsUrl);
    
    websocket.onopen = () => {
      console.log('WebSocket connected');
    };
    
    websocket.onmessage = (event) => {
      const message = JSON.parse(event.data);
      if (message.from_user === userId) {
        setMessages(previousMessages =>
          GiftedChat.append(previousMessages, [{
            _id: message.id,
            text: message.content,
            createdAt: new Date(message.timestamp),
            user: { _id: message.from_user },
          }])
        );
      }
    };
    
    websocket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    setWs(websocket);
  };

  const onSend = async (newMessages = []) => {
    const message = newMessages[0];
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        to_user: userId,
        content: message.text,
        type: 'text',
      }));
    }
    
    setMessages(previousMessages =>
      GiftedChat.append(previousMessages, message)
    );
  };

  return (
    <SafeAreaView style={styles.container}>
      <GiftedChat
        messages={messages}
        onSend={onSend}
        user={{ _id: currentUserId }}
        renderBubble={(props) => (
          <Bubble
            {...props}
            wrapperStyle={{
              right: { backgroundColor: '#DCF8C6' },
              left: { backgroundColor: '#FFFFFF' },
            }}
          />
        )}
        renderSend={(props) => (
          <Send {...props}>
            <View style={styles.sendButton}>
              <Icon name="send" size={24} color="#075E54" />
            </View>
          </Send>
        )}
      />
    </SafeAreaView>
  );
}

export default function App() {
  return (
    <NavigationContainer>
      <Stack.Navigator initialRouteName="Login">
        <Stack.Screen name="Login" component={LoginScreen} options={{ headerShown: false }} />
        <Stack.Screen name="Register" component={RegisterScreen} options={{ headerShown: false }} />
        <Stack.Screen name="ChatList" component={ChatListScreen} options={{ headerShown: false }} />
        <Stack.Screen name="Chat" component={ChatScreen} options={{ title: '' }} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  header: {
    alignItems: 'center',
    marginTop: 50,
    marginBottom: 30,
  },
  title: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#075E54',
  },
  subtitle: {
    fontSize: 16,
    color: '#666',
    marginTop: 10,
  },
  form: {
    paddingHorizontal: 20,
  },
  input: {
    borderWidth: 1,
    borderColor: '#ddd',
    borderRadius: 8,
    padding: 12,
    marginBottom: 15,
    fontSize: 16,
  },
  button: {
    backgroundColor: '#075E54',
    padding: 15,
    borderRadius: 8,
    alignItems: 'center',
    marginBottom: 15,
  },
  buttonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: 'bold',
  },
  linkText: {
    color: '#075E54',
    textAlign: 'center',
    fontSize: 14,
  },
  chatListHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 15,
    backgroundColor: '#075E54',
  },
  headerTitle: {
    fontSize: 20,
    fontWeight: 'bold',
    color: '#fff',
  },
  userItem: {
    flexDirection: 'row',
    padding: 15,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  userAvatar: {
    width: 50,
    height: 50,
    borderRadius: 25,
    backgroundColor: '#075E54',
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 15,
  },
  userInfo: {
    flex: 1,
    justifyContent: 'center',
  },
  userName: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 4,
  },
  userPhone: {
    fontSize: 14,
    color: '#666',
  },
  loadingText: {
    textAlign: 'center',
    marginTop: 50,
    fontSize: 16,
  },
  emptyText: {
    textAlign: 'center',
    marginTop: 50,
    fontSize: 16,
    color: '#666',
  },
  sendButton: {
    marginRight: 10,
    marginBottom: 5,
  },
});

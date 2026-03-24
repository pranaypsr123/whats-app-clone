# Build stage
FROM node:18-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm install

# Copy source code
COPY . .

# Build the web version
RUN npm run build:web

# Production stage
FROM nginx:alpine

# Copy built files to nginx
COPY --from=builder /app/web-build /usr/share/nginx/html

# Copy nginx configuration

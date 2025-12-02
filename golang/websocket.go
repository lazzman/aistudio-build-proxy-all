package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Constants for WebSocket
const (
	wsReadTimeout = 60 * time.Second
)

// WSMessage 是前后端之间通信的基本结构
type WSMessage struct {
	ID      string                 `json:"id"`      // 请求/响应的唯一ID
	Type    string                 `json:"type"`    // ping, pong, http_request, http_response, stream_start, stream_chunk, stream_end, error
	Payload map[string]interface{} `json:"payload"` // 具体数据
}

// pendingRequests 存储待处理的HTTP请求，等待WS响应
// key: reqID (string), value: chan *WSMessage
var pendingRequests sync.Map

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 生产环境中应设置严格的CheckOrigin
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 认证
	authToken := r.URL.Query().Get("auth_token")
	userID, err := validateJWT(authToken)
	if err != nil {
		log.Printf("WebSocket authentication failed: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 升级连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}

	// 添加到连接池
	userConn := globalPool.AddConnection(userID, conn)

	// 启动读取循环
	go readPump(userConn)
}

// readPump 处理来自单个WebSocket连接的所有传入消息
func readPump(uc *UserConnection) {
	defer func() {
		globalPool.RemoveConnection(uc.UserID, uc.Conn)
		uc.Conn.Close()
		log.Printf("readPump closed for user %s", uc.UserID)
	}()

	// 设置读取超时 (心跳机制)
	uc.Conn.SetReadDeadline(time.Now().Add(wsReadTimeout))

	for {
		_, message, err := uc.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for user %s: %v", uc.UserID, err)
			} else {
				log.Printf("WebSocket closed for user %s: %v", uc.UserID, err)
			}
			// 如果读取失败（包括超时），退出循环并清理连接
			break
		}

		// 收到任何消息，重置读取超时
		uc.Conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
		uc.LastActive = time.Now()

		// 解析消息
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshalling WebSocket message: %v", err)
			continue
		}

		switch msg.Type {
		case "ping":
			// 心跳响应
			err := uc.safeWriteJSON(map[string]string{"type": "pong", "id": msg.ID})
			if err != nil {
				log.Printf("Error sending pong: %v", err)
				return // 发送失败，认为连接已断
			}
		case "http_response", "stream_start", "stream_chunk", "stream_end", "error":
			// VERBOSE LOGGING: Log every message received from browser client
			log.Printf("[WS RECEIVED] Type: %s, ID: %s, User: %s", msg.Type, msg.ID, uc.UserID)
			if msg.Type == "http_response" || msg.Type == "error" {
				if status, ok := msg.Payload["status"].(float64); ok {
					log.Printf("[WS RECEIVED] Status code: %d", int(status))
				}
				if body, ok := msg.Payload["body"].(string); ok {
					log.Printf("[WS RECEIVED] Body length: %d bytes", len(body))
				}
			}

			// 路由响应到等待的HTTP Handler
			if ch, ok := pendingRequests.Load(msg.ID); ok {
				respChan := ch.(chan *WSMessage)
				// 尝试发送，如果通道已满（不太可能，但为了安全），则记录日志
				select {
				case respChan <- &msg:
					log.Printf("[WS RECEIVED] Successfully routed to pending request: %s", msg.ID)
				default:
					log.Printf("Warning: Response channel full for request ID %s, dropping message type %s", msg.ID, msg.Type)
				}
			} else {
				log.Printf("Received response for unknown or timed-out request ID: %s", msg.ID)
			}
		default:
			log.Printf("Received unknown message type from client: %s", msg.Type)
		}
	}
}

// validateJWT 模拟JWT验证并返回userID
func validateJWT(token string) (string, error) {
	if token == "" {
		return "", errors.New("missing auth_token")
	}
	// 实际应用中，这里需要使用JWT库（如golang-jwt/jwt）来验证签名和过期时间
	// 这里我们简单地将token当作userID
	if token == "valid-token-user-1" {
		return "user-1", nil
	}
	//if token == "valid-token-user-2" {
	//	return "user-2", nil
	//}
	return "", errors.New("invalid token")
}

// authenticateHTTPRequest 模拟HTTP代理请求的认证
func authenticateHTTPRequest(r *http.Request) (string, error) {
	// 实际应用中，可能检查Authorization头或其他API Key
	apiKey := r.Header.Get("x-goog-api-key")
	if apiKey == "" {
		// r.URL.Query() 会解析URL中的查询参数，返回一个 map[string][]string
		// .Get() 方法可以方便地获取指定参数的第一个值，如果参数不存在则返回空字符串
		apiKey = r.URL.Query().Get("key")
	}

	// 从环境变量中获取预期的API密钥
	expectedAPIKey := os.Getenv("AUTH_API_KEY")
	if expectedAPIKey == "" {
		log.Println("CRITICAL: AUTH_API_KEY environment variable not set.")
		// 在生产环境中，您可能希望完全阻止请求
		return "", errors.New("server configuration error")
	}

	if apiKey == expectedAPIKey {
		// 单租户
		return "user-1", nil
	}

	return "", errors.New("invalid API key")
}

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	proxyRequestTimeout = 600 * time.Second
)

func handleProxyRequest(w http.ResponseWriter, r *http.Request) {
	// 1. 认证并获取UserID (这里模拟)
	userID, err := authenticateHTTPRequest(r)
	if err != nil {
		http.Error(w, "Proxy authentication failed", http.StatusUnauthorized)
		return
	}

	// 2. 生成唯一请求ID
	reqID := uuid.NewString()

	// 3. 创建响应通道并注册
	// 使用带缓冲的通道以适应流式响应块
	respChan := make(chan *WSMessage, 10)
	pendingRequests.Store(reqID, respChan)
	defer pendingRequests.Delete(reqID) // 确保请求结束后清理

	// 4. 选择一个WebSocket连接
	selectedConn, err := globalPool.GetConnection(userID)
	if err != nil {
		log.Printf("Error getting connection for user %s: %v", userID, err)
		http.Error(w, "Service Unavailable: No active client connected", http.StatusServiceUnavailable)
		return
	}

	// 5. 封装HTTP请求为WS消息
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Fix tool definitions format for Gemini API compatibility
	// Roo/Cline sends "parametersJsonSchema" but Gemini expects "parameters"
	bodyBytes = fixToolDefinitions(bodyBytes)

	// Fix systemInstruction role field (should not have role: "user")
	bodyBytes = fixSystemInstruction(bodyBytes)

	// 注意：将Header直接序列化为JSON可能需要一些处理，这里简化处理
	// 对于生产环境，可能需要更精细的Header转换
	headers := make(map[string][]string)
	for k, v := range r.Header {
		// 过滤掉一些HTTP/1.1特有的或代理不应转发的头
		if k != "Connection" && k != "Keep-Alive" && k != "Proxy-Authenticate" && k != "Proxy-Authorization" && k != "Te" && k != "Trailers" && k != "Transfer-Encoding" && k != "Upgrade" {
			headers[k] = v
		}
	}

	requestPayload := WSMessage{
		ID:   reqID,
		Type: "http_request",
		Payload: map[string]interface{}{
			"method": r.Method,
			// 假设前端知道如何处理这个相对URL，或者您在这里构建完整的外部URL
			"url":     "https://generativelanguage.googleapis.com" + r.URL.String(),
			"headers": headers,
			"body":    string(bodyBytes), // 对于二进制数据，应使用base64编码
		},
	}

	// Concise stdout logging, full details in web UI
	log.Printf("[REQUEST %s] %s %s (%d bytes)", reqID, r.Method, r.URL.String(), len(bodyBytes))
	addLog("INFO", fmt.Sprintf("[REQUEST %s] %s %s", reqID, r.Method, r.URL.String()), map[string]interface{}{
		"request_id": reqID,
		"method":     r.Method,
		"url":        r.URL.String(),
		"headers":    headers,
		"body":       string(bodyBytes),
	})

	// 6. 发送请求到WebSocket客户端
	if err := selectedConn.safeWriteJSON(requestPayload); err != nil {
		errMsg := fmt.Sprintf("[ERROR %s] Failed to send request over WebSocket: %v", reqID, err)
		log.Println(errMsg)
		addLog("ERROR", errMsg, map[string]interface{}{
			"request_id": reqID,
			"error":      err.Error(),
		})
		http.Error(w, "Bad Gateway: Failed to send request to client", http.StatusBadGateway)
		return
	}
	successMsg := fmt.Sprintf("[REQUEST %s] Sent to WebSocket client", reqID)
	log.Println(successMsg)
	addLog("INFO", successMsg, map[string]interface{}{"request_id": reqID})

	// 7. 异步等待并处理响应
	processWebSocketResponse(w, r, respChan)
}

// processWebSocketResponse 处理来自WS通道的响应，构建HTTP响应
func processWebSocketResponse(w http.ResponseWriter, r *http.Request, respChan chan *WSMessage) {
	// 设置超时
	ctx, cancel := context.WithTimeout(r.Context(), proxyRequestTimeout)
	defer cancel()

	// 获取Flusher以支持流式响应
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("Warning: ResponseWriter does not support flushing, streaming will be buffered.")
	}

	headersSet := false
	var errorBodyChunks []string
	var errorStatusCode int
	var errorRequestID string

	for {
		select {
		case msg, ok := <-respChan:
			if !ok {
				// 通道被关闭，理论上不应该发生，除非有panic
				if !headersSet {
					http.Error(w, "Internal Server Error: Response channel closed unexpectedly", http.StatusInternalServerError)
				}
				return
			}

			switch msg.Type {
			case "http_response":
				// 标准单个响应
				if headersSet {
					log.Println("Received http_response after headers were already set. Ignoring.")
					return
				}

				// Extract request ID from context if available
				reqID := ""
				if id, ok := msg.Payload["request_id"].(string); ok {
					reqID = id
				} else {
					reqID = msg.ID
				}

				statusCode := 0
				if status, ok := msg.Payload["status"].(float64); ok {
					statusCode = int(status)
				}

				bodyLen := 0
				if body, ok := msg.Payload["body"].(string); ok {
					bodyLen = len(body)
				}

				// Concise stdout logging, full details in web UI
				log.Printf("[RESPONSE %s] Status: %d (%d bytes)", reqID, statusCode, bodyLen)
				addLog("INFO", fmt.Sprintf("[RESPONSE %s] Status: %d", reqID, statusCode), map[string]interface{}{
					"request_id": reqID,
					"status":     statusCode,
					"headers":    msg.Payload["headers"],
					"body":       msg.Payload["body"],
				})

				setResponseHeaders(w, msg.Payload)
				writeStatusCode(w, msg.Payload)
				writeBody(w, msg.Payload)
				return // 请求结束

			case "stream_start":
				// 流开始
				if headersSet {
					log.Println("Received stream_start after headers were already set. Ignoring.")
					continue
				}

				// Extract request ID and status
				reqID := ""
				if id, ok := msg.Payload["request_id"].(string); ok {
					reqID = id
				} else {
					reqID = msg.ID
				}
				statusCode := 0
				if status, ok := msg.Payload["status"].(float64); ok {
					statusCode = int(status)
				}

				log.Printf("[STREAM] Starting (Status: %v)", msg.Payload["status"])

				// Log error status codes to structured logs for debugging
				if statusCode >= 400 {
					errorStatusCode = statusCode
					errorRequestID = reqID
					errorBodyChunks = []string{}
					addLog("WARN", fmt.Sprintf("[STREAM ERROR %s] Status: %d - Waiting for error body in chunks", reqID, statusCode), map[string]interface{}{
						"request_id": reqID,
						"status":     statusCode,
						"headers":    msg.Payload["headers"],
					})
				}

				setResponseHeaders(w, msg.Payload)
				writeStatusCode(w, msg.Payload)
				headersSet = true
				if flusher != nil {
					flusher.Flush()
				}

			case "stream_chunk":
				// 流数据块 - no stdout logging for chunks to reduce noise
				if !headersSet {
					log.Println("Warning: Received stream_chunk before stream_start. Using default 200 OK.")
					w.WriteHeader(http.StatusOK)
					headersSet = true
				}

				// If this is an error response, accumulate chunks for logging
				if errorStatusCode >= 400 {
					if data, ok := msg.Payload["data"].(string); ok {
						errorBodyChunks = append(errorBodyChunks, data)
					}
				}

				writeBody(w, msg.Payload)
				if flusher != nil {
					flusher.Flush()
				}

			case "stream_end":
				// 流结束
				if !headersSet {
					w.WriteHeader(http.StatusOK)
				}

				// If this was an error response, log the complete error body
				if errorStatusCode >= 400 && len(errorBodyChunks) > 0 {
					fullErrorBody := ""
					for _, chunk := range errorBodyChunks {
						fullErrorBody += chunk
					}
					addLog("ERROR", fmt.Sprintf("[STREAM ERROR %s] Complete error response from Gemini API", errorRequestID), map[string]interface{}{
						"request_id":  errorRequestID,
						"status":      errorStatusCode,
						"error_body":  fullErrorBody,
					})
					log.Printf("[STREAM ERROR] %s - Status %d - Body: %s", errorRequestID, errorStatusCode, fullErrorBody)
				}

				log.Println("[STREAM] Completed")
				return

			case "error":
				// 前端返回错误
				if !headersSet {
					// Extract request ID
					reqID := ""
					if id, ok := msg.Payload["request_id"].(string); ok {
						reqID = id
					} else {
						reqID = msg.ID
					}

					errMsg := "Bad Gateway: Client reported an error"
					if payloadErr, ok := msg.Payload["error"].(string); ok {
						errMsg = payloadErr
					}
					statusCode := http.StatusBadGateway
					if code, ok := msg.Payload["status"].(float64); ok {
						statusCode = int(code)
					}

					// Concise stdout logging, full details in web UI
					log.Printf("[ERROR %s] Status: %d - %s", reqID, statusCode, errMsg)
					addLog("ERROR", fmt.Sprintf("[ERROR %s] Status: %d", reqID, statusCode), map[string]interface{}{
						"request_id": reqID,
						"status":     statusCode,
						"error":      errMsg,
						"headers":    msg.Payload["headers"],
						"body":       msg.Payload["body"],
						"url":        msg.Payload["url"],
						"method":     msg.Payload["method"],
						"payload":    msg.Payload,
					})
					http.Error(w, errMsg, statusCode)
				} else {
					// 如果已经开始发送流，我们只能记录错误并关闭连接
					log.Printf("[ERROR] Error received from client after stream started: %v", msg.Payload)
				}
				return // 请求结束

			default:
				log.Printf("[UNKNOWN] Received unexpected message type %s while waiting for response", msg.Type)
			}

		case <-ctx.Done():
			// 超时
			if !headersSet {
				log.Printf("Gateway Timeout: No response from client for request %s", r.URL.Path)
				http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
			} else {
				// 如果流已经开始，我们只能记录日志并断开连接
				log.Printf("Gateway Timeout: Stream incomplete for request %s", r.URL.Path)
			}
			return
		}
	}
}

// --- 辅助函数 ---

// setResponseHeaders 从payload中解析并设置HTTP响应头
func setResponseHeaders(w http.ResponseWriter, payload map[string]interface{}) {
	headers, ok := payload["headers"].(map[string]interface{})
	if !ok {
		return
	}
	for key, value := range headers {
		// 假设值是 []interface{} 或 string
		if values, ok := value.([]interface{}); ok {
			for _, v := range values {
				if strV, ok := v.(string); ok {
					w.Header().Add(key, strV)
				}
			}
		} else if strV, ok := value.(string); ok {
			w.Header().Set(key, strV)
		}
	}
}

// writeStatusCode 从payload中解析并设置HTTP状态码
func writeStatusCode(w http.ResponseWriter, payload map[string]interface{}) {
	status, ok := payload["status"].(float64) // JSON数字默认为float64
	if !ok {
		w.WriteHeader(http.StatusOK) // 默认200
		return
	}
	w.WriteHeader(int(status))
}

// writeBody 从payload中解析并写入HTTP响应体
func writeBody(w http.ResponseWriter, payload map[string]interface{}) {
	var bodyData []byte
	// 对于 http_response，body 键通常包含数据
	if body, ok := payload["body"].(string); ok {
		bodyData = []byte(body)
	}
	// 对于 stream_chunk，data 键通常包含数据
	if data, ok := payload["data"].(string); ok {
		bodyData = []byte(data)
	}
	// 注意：如果前端发送的是二进制数据，这里应该假设它是base64编码的字符串并进行解码

	if len(bodyData) > 0 {
		w.Write(bodyData)
	}
}

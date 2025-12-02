package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// --- Constants ---
const (
	wsPath          = "/v1/ws"
	proxyListenAddr = ":5345"
)

// --- Global Connection Pool ---
var globalPool = &ConnectionPool{
	Users: make(map[string]*UserConnections),
}

// --- Logs and Health API Endpoints ---

func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for local development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	logBufferMu.RLock()
	defer logBufferMu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logBuffer,
		"count": len(logBuffer),
	})
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	globalPool.RLock()
	userCount := len(globalPool.Users)
	totalConns := 0
	for _, userConns := range globalPool.Users {
		userConns.Lock()
		totalConns += len(userConns.Connections)
		userConns.Unlock()
	}
	globalPool.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":             "healthy",
		"timestamp":          time.Now(),
		"active_users":       userCount,
		"active_connections": totalConns,
		"log_buffer_size":    len(logBuffer),
	})
}

// --- Main Function ---

func main() {
	// WebSocket 路由
	http.HandleFunc(wsPath, handleWebSocket)

	// Logs and Health API routes (no auth required for monitoring)
	http.HandleFunc("/api/logs", handleGetLogs)
	http.HandleFunc("/api/health", handleHealthCheck)

	// Log viewer UI (static files - no auth required)
	// Use a custom handler to serve static files without authentication
	http.HandleFunc("/logs-ui/", func(w http.ResponseWriter, r *http.Request) {
		// Serve the static files from log-viewer/dist
		fs := http.FileServer(http.Dir("./log-viewer/dist"))
		// Strip the /logs-ui/ prefix before serving
		http.StripPrefix("/logs-ui/", fs).ServeHTTP(w, r)
	})

	// Serve static assets (CSS, JS) without /logs-ui/ prefix for correct paths
	http.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		// Serve assets from log-viewer/dist/assets
		fs := http.FileServer(http.Dir("./log-viewer/dist"))
		fs.ServeHTTP(w, r)
	})

	// HTTP 反向代理路由 (捕获所有其他请求)
	http.HandleFunc("/", handleProxyRequest)

	log.Printf("Starting server on %s", proxyListenAddr)
	log.Printf("WebSocket endpoint available at ws://%s%s", proxyListenAddr, wsPath)
	log.Printf("HTTP proxy available at http://%s/", proxyListenAddr)
	log.Printf("Log viewer UI available at http://%s/logs-ui/", proxyListenAddr)

	if err := http.ListenAndServe(proxyListenAddr, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

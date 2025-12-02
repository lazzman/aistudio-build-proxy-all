package main

import (
	"sync"
	"time"
)

// LogEntry represents a single log entry for the web UI
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"` // INFO, ERROR, WARN, DEBUG
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

var (
	logBuffer    []LogEntry
	logBufferMu  sync.RWMutex
	maxLogBuffer = 1000 // Keep last 1000 log entries
)

// addLog adds a log entry to the buffer
func addLog(level, message string, data map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Data:      data,
	}

	logBufferMu.Lock()
	defer logBufferMu.Unlock()

	logBuffer = append(logBuffer, entry)
	if len(logBuffer) > maxLogBuffer {
		logBuffer = logBuffer[1:] // Remove oldest entry
	}
}

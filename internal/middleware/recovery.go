package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error   string    `json:"error"`
	Code    string    `json:"code"`
	Request string    `json:"request_id,omitempty"`
	Time    time.Time `json:"timestamp"`
}

type PanicLogEntry struct {
	RequestID   string        `json:"request_id"`
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	RemoteAddr  string        `json:"remote_addr"`
	UserAgent   string        `json:"user_agent"`
	Panic       string        `json:"panic"`
	Stack       string        `json:"stack"`
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		defer func() {
			if err := recover(); err != nil {
				duration := time.Since(start)
				
				// Log detailed panic information
				logPanic(c, requestID, err, duration)
				
				// Check if response has already been written
				if c.Writer.Written() {
					// Can't write safe response if headers already sent
					log.Printf("PANIC RECOVERY: Cannot write safe response - headers already sent for request %s", requestID)
					return
				}

				// Return safe error response to client
				sendSafeErrorResponse(c, requestID)
				c.Abort()
			}
		}()

		c.Next()
	}
}

func logPanic(c *gin.Context, requestID string, panicErr interface{}, duration time.Duration) {
	stack := string(debug.Stack())
	
	// Sanitize the stack trace for logging
	stack = sanitizeStack(stack)
	
	entry := PanicLogEntry{
		RequestID:   requestID,
		Method:      c.Request.Method,
		Path:        c.Request.URL.Path,
		RemoteAddr:  c.Request.RemoteAddr,
		UserAgent:   c.Request.UserAgent(),
		Panic:       fmt.Sprintf("%v", panicErr),
		Stack:       stack,
		Timestamp:   time.Now(),
		Duration:    duration,
	}

	// Convert to JSON for structured logging
	logJSON, _ := json.Marshal(entry)
	log.Printf("PANIC RECOVERED: %s", string(logJSON))
}

func sanitizeStack(stack string) string {
	// Remove sensitive information from stack trace if any
	// For now, just limit the length to prevent log flooding
	if len(stack) > 4000 {
		return stack[:4000] + "\n... (truncated)"
	}
	return stack
}

func sendSafeErrorResponse(c *gin.Context, requestID string) {
	// Don't expose panic details to client
	errorResp := ErrorResponse{
		Error:   "Internal server error",
		Code:    "INTERNAL_ERROR",
		Request: requestID,
		Time:    time.Now(),
	}

	// Try to send JSON response
	if c.GetHeader("Accept") == "application/json" || 
	   c.GetHeader("Content-Type") == "application/json" {
		c.JSON(500, errorResp)
	} else {
		// Fallback to plain text
		c.Header("Content-Type", "text/plain")
		c.String(500, "Internal Server Error\nRequest ID: %s\n", requestID)
	}
}

// RequestID middleware to ensure request ID is available for all handlers
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Helper function to get request ID from context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

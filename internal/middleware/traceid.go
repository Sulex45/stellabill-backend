package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceIDMiddleware injects a trace ID into the request context for observability
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if trace ID already exists in the header
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			// Generate a new trace ID if not provided
			traceID = uuid.New().String()
		}

		// Set trace ID in context
		c.Set("traceID", traceID)

		// Pass trace ID to response headers for client tracking
		c.Header("X-Trace-ID", traceID)

		c.Next()
	}
}

package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"stellarbill-backend/internal/security"
)

// Logger returns a gin middleware that logs requests using zap with PII redaction
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log completed request
		redactedPath := security.MaskPII(c.Request.URL.Path)
		redactedClientIP := security.MaskPII(c.ClientIP())
		latency := time.Since(start)

		logger.Info("HTTP request completed",
			zap.String("method", c.Request.Method),
			zap.String("path", redactedPath),
			zap.String("client_ip", redactedClientIP),
			zap.Int("status", c.Writer.Status()),
			zap.String("latency", latency.String()),
			zap.Int("bytes_written", c.Writer.Size()),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}


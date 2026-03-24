package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"stellarbill-backend/internal/config"
	"stellarbill-backend/internal/middleware"
	"stellarbill-backend/internal/routes"
	"stellarbill-backend/internal/security"
)

func main() {
	// Load configuration with strict validation
	cfg, err := config.Load()
	if err != nil {
		// Fail fast with descriptive error
		fmt.Fprintf(os.Stderr, "ERROR: Configuration validation failed: %s\n", err.Error())
		fmt.Fprintln(os.Stderr, "\nRequired environment variables:")
		for _, key := range config.GetRequiredEnvVars() {
			fmt.Fprintf(os.Stderr, "  - %s\n", key)
		}
		fmt.Fprintln(os.Stderr, "\nOptional environment variables and defaults:")
		for key, val := range config.GetOptionalEnvVars() {
			fmt.Fprintf(os.Stderr, "  - %s (default: %s)\n", key, val)
		}
		os.Exit(1)
	}

	// Init PII-safe logger
	var logger *zap.Logger
	if cfg.Env == "production" {
		logger = security.ProductionLogger()
		defer logger.Sync()
		gin.SetMode(gin.ReleaseMode)
		logger.Info("Running in production mode")
	} else if cfg.Env == "development" {
		logger = security.DevLogger()
		defer logger.Sync()
		gin.SetMode(gin.DebugMode)
		logger.Info("Running in development mode")
	} else {
		logger = security.ProductionLogger()
		defer logger.Sync()
		gin.SetMode(gin.TestMode)
		logger.Info("Running in test mode", zap.String("env", cfg.Env))
	}

	// Log config warnings
	if vResult := cfg.Validate(); len(vResult.Warnings) > 0 {
		logger.Warn("Configuration warnings",
			zap.Strings("warnings", vResult.Warnings))
	}

	// Create router with configured timeouts
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))

	// Set security headers
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	// Register routes
	routes.Register(router)

	// Build server address
	addr := fmt.Sprintf(":%d", cfg.Port)

	// Create HTTP server with configuration
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
	}

	logger.Info("Starting Stellarbill backend",
		zap.String("addr", addr),
		zap.String("env", cfg.Env))
	logger.Info("Server timeouts",
		zap.Int("read", cfg.ReadTimeout),
		zap.Int("write", cfg.WriteTimeout),
		zap.Int("idle", cfg.IdleTimeout))

	// Start server with fail-fast behavior
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}


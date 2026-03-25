package routes

import (
	"os"

	"github.com/gin-gonic/gin"
	"stellarbill-backend/internal/config"
	"stellarbill-backend/internal/handlers"
	"stellarbill-backend/internal/middleware"
	"stellarbill-backend/internal/repository"
	"stellarbill-backend/internal/service"
)

func Register(r *gin.Engine) {
	r.Use(corsMiddleware())

	// Load configuration for rate limiting
	cfg, err := config.Load()
	if err != nil {
		// Use defaults if config loading fails
		cfg = config.Config{
			RateLimitEnabled: true,
			RateLimitMode:    "ip",
			RateLimitRPS:     10,
			RateLimitBurst:   20,
			RateLimitWhitelist: []string{"/api/health"},
		}
	}

	// Apply rate limiting middleware
	rateLimitConfig := middleware.RateLimiterConfig{
		Enabled:        cfg.RateLimitEnabled,
		Mode:           middleware.RateLimitMode(cfg.RateLimitMode),
		RequestsPerSec: int64(cfg.RateLimitRPS),
		BurstSize:      int64(cfg.RateLimitBurst),
		WhitelistPaths: cfg.RateLimitWhitelist,
	}
	r.Use(middleware.RateLimitMiddleware(rateLimitConfig))

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret"
	}

	subRepo := repository.NewMockSubscriptionRepo()
	planRepo := repository.NewMockPlanRepo()
	svc := service.NewSubscriptionService(subRepo, planRepo)

	api := r.Group("/api")
	{
		api.GET("/health", handlers.Health)
		api.GET("/subscriptions", handlers.ListSubscriptions)
		api.GET("/subscriptions/:id", middleware.AuthMiddleware(jwtSecret), handlers.NewGetSubscriptionHandler(svc))
		api.GET("/plans", handlers.ListPlans)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

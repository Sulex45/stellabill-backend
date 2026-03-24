package config

import "os"

type Config struct {
	Env                string
	Port               string
	DBConn             string
	JWTSecret          string
	SecurityHSTSMaxAge string
	SecurityFrameOpt   string
}

func Load() Config {
	return Config{
		Env:                getEnv("ENV", "development"),
		Port:               getEnv("PORT", "8080"),
		DBConn:             getEnv("DATABASE_URL", "postgres://localhost/stellarbill?sslmode=disable"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		SecurityHSTSMaxAge: getEnv("SECURITY_HSTS_MAX_AGE", "31536000"),
		SecurityFrameOpt:   getEnv("SECURITY_FRAME_OPTIONS", "DENY"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

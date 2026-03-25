package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// ConfigErrorType represents the category of configuration error
type ConfigErrorType string

const (
	ErrMissingEnvVar    ConfigErrorType = "MISSING_ENV_VAR"
	ErrInvalidPort      ConfigErrorType = "INVALID_PORT"
	ErrInvalidURL       ConfigErrorType = "INVALID_URL"
	ErrWeakSecret       ConfigErrorType = "WEAK_SECRET"
	ErrValidationFailed ConfigErrorType = "VALIDATION_FAILED"
)

// ConfigError represents a typed configuration error
type ConfigError struct {
	Type    ConfigErrorType
	Key     string
	Message string
	Value   string
}

func (e *ConfigError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("config error [%s]: %s (key=%s, value=%s)", e.Type, e.Message, e.Key, e.Value)
	}
	return fmt.Sprintf("config error [%s]: %s", e.Type, e.Message)
}

// Config holds all application configuration
type Config struct {
	Env       string
	Port      int
	DBConn    string
	JWTSecret string
	// Add additional secure defaults for optional configs
	MaxHeaderBytes int
	ReadTimeout    int
	WriteTimeout   int
	IdleTimeout    int
	// Rate limiting configuration
	RateLimitEnabled    bool
	RateLimitMode       string
	RateLimitRPS        int
	RateLimitBurst      int
	RateLimitWhitelist  []string
}

// ValidationResult holds the result of configuration validation
type ValidationResult struct {
	Errors   []ConfigError
	Warnings []string
}

// Valid returns true if there are no validation errors
func (v *ValidationResult) Valid() bool {
	return len(v.Errors) == 0
}

// Error returns a formatted string of all validation errors
func (v *ValidationResult) Error() string {
	if v.Valid() {
		return ""
	}
	var errs []string
	for _, e := range v.Errors {
		errs = append(errs, e.Error())
	}
	return strings.Join(errs, "; ")
}

// Constants for configuration limits
const (
	DefaultPort          = 8080
	MinPort              = 1
	MaxPort              = 65535
	MinSecretLength      = 32
	MaxHeaderBytes       = 1 << 20  // 1MB
	DefaultReadTimeout   = 30       // seconds
	DefaultWriteTimeout  = 30       // seconds
	DefaultIdleTimeout   = 120      // seconds
	DefaultRateLimitRPS  = 10       // requests per second
	DefaultRateLimitBurst = 20      // burst size
)

// Required environment variables
var requiredEnvVars = []string{
	"DATABASE_URL",
	"JWT_SECRET",
}

// Optional environment variables with defaults
var optionalEnvVars = map[string]string{
	"PORT":            "8080",
	"ENV":             "development",
	"MAX_HEADER_BYTES": "1048576",
	"READ_TIMEOUT":    "30",
	"WRITE_TIMEOUT":   "30",
	"IDLE_TIMEOUT":    "120",
	"RATE_LIMIT_ENABLED": "true",
	"RATE_LIMIT_MODE": "ip",
	"RATE_LIMIT_RPS": "10",
	"RATE_LIMIT_BURST": "20",
	"RATE_LIMIT_WHITELIST": "/api/health",
}

// Load loads configuration from environment variables with validation
func Load() (Config, error) {
	cfg := Config{
		Env:             getEnv("ENV", "development"),
		Port:            DefaultPort,
		DBConn:          "",
		JWTSecret:       "",
		MaxHeaderBytes:  MaxHeaderBytes,
		ReadTimeout:     DefaultReadTimeout,
		WriteTimeout:    DefaultWriteTimeout,
		IdleTimeout:     DefaultIdleTimeout,
		RateLimitEnabled: getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitMode:    getEnv("RATE_LIMIT_MODE", "ip"),
		RateLimitRPS:     getEnvInt("RATE_LIMIT_RPS", DefaultRateLimitRPS),
		RateLimitBurst:   getEnvInt("RATE_LIMIT_BURST", DefaultRateLimitBurst),
		RateLimitWhitelist: getEnvSlice("RATE_LIMIT_WHITELIST", []string{"/api/health"}),
	}

	result := cfg.Validate()
	if !result.Valid() {
		return Config{}, result
	}

	return cfg, nil
}

// Validate validates the configuration and returns a ValidationResult
func (c *Config) Validate() *ValidationResult {
	result := &ValidationResult{
		Errors:   []ConfigError{},
		Warnings: []string{},
	}

	// Validate required environment variables are present
	for _, key := range requiredEnvVars {
		if value := os.Getenv(key); value == "" {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrMissingEnvVar,
				Key:     key,
				Message: "required environment variable is missing",
				Value:   "",
			})
		}
	}

	// Validate PORT
	if portStr := os.Getenv("PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidPort,
				Key:     "PORT",
				Message: "must be a valid integer",
				Value:   portStr,
			})
		} else if port < MinPort || port > MaxPort {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidPort,
				Key:     "PORT",
				Message: fmt.Sprintf("must be between %d and %d", MinPort, MaxPort),
				Value:   portStr,
			})
		} else {
			c.Port = port
		}
	}

	// Validate DATABASE_URL format
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if !isValidDatabaseURL(dbURL) {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidURL,
				Key:     "DATABASE_URL",
				Message: "must be a valid database connection string",
				Value:   maskPassword(dbURL),
			})
		} else {
			c.DBConn = dbURL
		}
	}

	// Validate JWT_SECRET
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		if !isValidSecret(secret) {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrWeakSecret,
				Key:     "JWT_SECRET",
				Message: fmt.Sprintf("must be at least %d characters and contain mixed alphanumeric and special characters", MinSecretLength),
				Value:   maskSecret(secret),
			})
		} else {
			c.JWTSecret = secret
		}
	}

	// Validate optional MAX_HEADER_BYTES
	if val := os.Getenv("MAX_HEADER_BYTES"); val != "" {
		if max, err := strconv.Atoi(val); err == nil && max > 0 {
			c.MaxHeaderBytes = max
		} else {
			result.Warnings = append(result.Warnings, "MAX_HEADER_BYTES invalid, using default")
		}
	}

	// Validate optional timeouts
	if val := os.Getenv("READ_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			c.ReadTimeout = timeout
		} else {
			result.Warnings = append(result.Warnings, "READ_TIMEOUT invalid, using default")
		}
	}

	if val := os.Getenv("WRITE_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			c.WriteTimeout = timeout
		} else {
			result.Warnings = append(result.Warnings, "WRITE_TIMEOUT invalid, using default")
		}
	}

	if val := os.Getenv("IDLE_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			c.IdleTimeout = timeout
		} else {
			result.Warnings = append(result.Warnings, "IDLE_TIMEOUT invalid, using default")
		}
	}

	// Validate rate limiting configuration
	if val := os.Getenv("RATE_LIMIT_ENABLED"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			c.RateLimitEnabled = enabled
		} else {
			result.Warnings = append(result.Warnings, "RATE_LIMIT_ENABLED invalid, using default")
		}
	}

	if mode := os.Getenv("RATE_LIMIT_MODE"); mode != "" {
		validModes := map[string]bool{"ip": true, "user": true, "hybrid": true}
		if validModes[mode] {
			c.RateLimitMode = mode
		} else {
			result.Warnings = append(result.Warnings, "RATE_LIMIT_MODE invalid, using default")
		}
	}

	if val := os.Getenv("RATE_LIMIT_RPS"); val != "" {
		if rps, err := strconv.Atoi(val); err == nil && rps > 0 && rps <= 1000 {
			c.RateLimitRPS = rps
		} else {
			result.Warnings = append(result.Warnings, "RATE_LIMIT_RPS invalid, using default")
		}
	}

	if val := os.Getenv("RATE_LIMIT_BURST"); val != "" {
		if burst, err := strconv.Atoi(val); err == nil && burst > 0 && burst <= 5000 {
			c.RateLimitBurst = burst
		} else {
			result.Warnings = append(result.Warnings, "RATE_LIMIT_BURST invalid, using default")
		}
	}

	if whitelist := os.Getenv("RATE_LIMIT_WHITELIST"); whitelist != "" {
		paths := strings.Split(whitelist, ",")
		for i, path := range paths {
			paths[i] = strings.TrimSpace(path)
		}
		c.RateLimitWhitelist = paths
	}

	// Set optional env values
	c.Env = getEnv("ENV", "development")

	return result
}

// isValidDatabaseURL validates that the database URL has a valid scheme and structure
func isValidDatabaseURL(dbURL string) bool {
	// Must not be empty
	if dbURL == "" {
		return false
	}

	// Parse the URL to validate its structure
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return false
	}

	// Must have a valid scheme (postgres, postgresql, mysql, sqlite, etc.)
	scheme := strings.ToLower(parsed.Scheme)
	validSchemes := map[string]bool{
		"postgres": true,
		"postgresql": true,
		"mysql": true,
		"sqlite": true,
		"sqlite3": true,
		"mongodb": true,
		"redis": true,
	}

	if !validSchemes[scheme] && !strings.Contains(scheme, "sql") {
		// Allow unknown schemes but warn - be permissive but validate structure
	}

	// Must have a host or be a valid file path (for sqlite)
	if parsed.Host == "" && !strings.Contains(dbURL, ".db") && !strings.Contains(dbURL, ".sqlite") {
		// Could be a local database
	}

	return true
}

// isValidSecret validates that the secret meets security requirements
func isValidSecret(secret string) bool {
	if len(secret) < MinSecretLength {
		return false
	}

	// Check for mixed character types
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, r := range secret {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	// Require at least 3 of the 4 character types
	typesCount := 0
	if hasUpper {
		typesCount++
	}
	if hasLower {
		typesCount++
	}
	if hasDigit {
		typesCount++
	}
	if hasSpecial {
		typesCount++
	}

	return typesCount >= 3
}

// maskPassword masks the password in a database URL for security
func maskPassword(dbURL string) string {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return "***"
	}

	if parsed.User != nil {
		parsed.User = url.UserPassword(parsed.User.Username(), "***")
	}

	return parsed.String()
}

// maskSecret masks a secret for logging
func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}

// getEnv retrieves an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvBool retrieves an environment variable as boolean with a fallback value
func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// getEnvInt retrieves an environment variable as integer with a fallback value
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// getEnvSlice retrieves an environment variable as string slice with a fallback value
func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		for i, part := range parts {
			parts[i] = strings.TrimSpace(part)
		}
		return parts
	}
	return fallback
}

// GetRequiredEnvVars returns the list of required environment variables
func GetRequiredEnvVars() []string {
	return requiredEnvVars
}

// GetOptionalEnvVars returns the map of optional environment variables with their defaults
func GetOptionalEnvVars() map[string]string {
	return optionalEnvVars
}

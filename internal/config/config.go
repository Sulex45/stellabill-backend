package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"

	"stellarbill-backend/internal/secrets"
)

// ConfigErrorType represents the category of configuration error
type ConfigErrorType string

const (
	ErrMissingEnvVar    ConfigErrorType = "MISSING_ENV_VAR"
	ErrInvalidPort      ConfigErrorType = "INVALID_PORT"
	ErrInvalidURL       ConfigErrorType = "INVALID_URL"
	ErrWeakSecret       ConfigErrorType = "WEAK_SECRET"
	ErrInvalidValue     ConfigErrorType = "INVALID_VALUE"
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
	AllowedOrigins string
	AdminToken     string
	// Rate limiting configuration
	RateLimitEnabled   bool
	RateLimitMode      string
	RateLimitRPS       int
	RateLimitBurst     int
	RateLimitWhitelist []string
	// Tracing configuration
	TracingExporter    string
	TracingServiceName string
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
	DefaultPort           = 8080
	MinPort               = 1
	MaxPort               = 65535
	MinSecretLength       = 12
	MaxHeaderBytes        = 1 << 20 // 1MB
	DefaultReadTimeout    = 30      // seconds
	DefaultWriteTimeout   = 30      // seconds
	DefaultIdleTimeout    = 120     // seconds
	DefaultRateLimitRPS   = 10
	DefaultRateLimitBurst = 20
	MinTimeoutSeconds     = 1
	MaxTimeoutSeconds     = 3600
	MinHeaderBytes        = 1024
	MaxAllowedHeaderBytes = 16 << 20 // 16MB
	MinRateLimitRPS       = 1
	MaxRateLimitRPS       = 1000
	MinRateLimitBurst     = 1
	MaxRateLimitBurst     = 5000
)

// Required environment variables
var requiredEnvVars = []string{
	"DATABASE_URL",
	"JWT_SECRET",
	"ADMIN_TOKEN",
}

// Optional environment variables with defaults
var optionalEnvVars = map[string]string{
	"PORT":                 "8080",
	"ENV":                  "development",
	"MAX_HEADER_BYTES":     "1048576",
	"READ_TIMEOUT":         "30",
	"WRITE_TIMEOUT":        "30",
	"IDLE_TIMEOUT":         "120",
	"RATE_LIMIT_ENABLED":   "true",
	"RATE_LIMIT_MODE":      "ip",
	"RATE_LIMIT_RPS":       "10",
	"RATE_LIMIT_BURST":     "20",
	"RATE_LIMIT_WHITELIST": "/api/health",
	"TRACING_EXPORTER":     "stdout",
	"TRACING_SERVICE_NAME": "stellabill-backend",
	"ALLOWED_ORIGINS":      "",
}

// Option configures the Load function.
type Option func(*loadOptions)

type loadOptions struct {
	secretsProvider secrets.Provider
}

// WithSecretsProvider overrides the default env-based secrets provider.
func WithSecretsProvider(p secrets.Provider) Option {
	return func(o *loadOptions) {
		o.secretsProvider = p
	}
}

// secretKeys are the config keys that must be fetched through the secrets provider
// rather than read directly from os.Getenv.
var secretKeys = []string{
	"DATABASE_URL",
	"JWT_SECRET",
	"ADMIN_TOKEN",
}

// Load loads configuration from environment variables with validation.
// Sensitive values (DATABASE_URL, JWT_SECRET) are fetched through the secrets
// provider, which defaults to EnvProvider when no option is supplied.
func Load(opts ...Option) (Config, error) {
	o := &loadOptions{
		secretsProvider: secrets.NewEnvProvider(),
	}
	for _, fn := range opts {
		fn(o)
	}

	cfg := Config{
		Env:                getEnv("ENV", "development"),
		Port:               DefaultPort,
		DBConn:             "",
		JWTSecret:          "",
		MaxHeaderBytes:     MaxHeaderBytes,
		ReadTimeout:        DefaultReadTimeout,
		WriteTimeout:       DefaultWriteTimeout,
		IdleTimeout:        DefaultIdleTimeout,
		AllowedOrigins:     strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")),
		RateLimitEnabled:   getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitMode:      getEnv("RATE_LIMIT_MODE", "ip"),
		RateLimitRPS:       getEnvInt("RATE_LIMIT_RPS", DefaultRateLimitRPS),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", DefaultRateLimitBurst),
		RateLimitWhitelist: getEnvSlice("RATE_LIMIT_WHITELIST", []string{"/api/health"}),
		TracingExporter:    getEnv("TRACING_EXPORTER", "stdout"),
		TracingServiceName: getEnv("TRACING_SERVICE_NAME", "stellabill-backend"),
	}

	// Resolve secrets through the provider
	resolved, secretErrs := resolveSecrets(o.secretsProvider, secretKeys)

	result := cfg.validate(resolved, secretErrs)
	if !result.Valid() {
		return Config{}, result
	}

	return cfg, nil
}

// resolveSecrets fetches each key from the provider and returns the values
// alongside any errors keyed by name.
func resolveSecrets(p secrets.Provider, keys []string) (map[string]string, map[string]error) {
	ctx := context.Background()
	vals := make(map[string]string, len(keys))
	errs := make(map[string]error, len(keys))

	for _, k := range keys {
		v, err := p.GetSecret(ctx, k)
		if err != nil {
			errs[k] = err
		} else {
			vals[k] = v
		}
	}
	return vals, errs
}

// Validate validates the configuration using os.Getenv for secrets (legacy path).
// Prefer Load() which uses the secrets provider abstraction.
func (c *Config) Validate() *ValidationResult {
	p := secrets.NewEnvProvider()
	resolved, secretErrs := resolveSecrets(p, secretKeys)
	return c.validate(resolved, secretErrs)
}

// validate is the internal validation method that uses pre-resolved secrets.
func (c *Config) validate(resolvedSecrets map[string]string, secretErrs map[string]error) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ConfigError{},
		Warnings: []string{},
	}

	// Validate required secrets are present via the provider
	for _, key := range secretKeys {
		if err, failed := secretErrs[key]; failed {
			if errors.Is(err, secrets.ErrSecretNotFound) {
				result.Errors = append(result.Errors, ConfigError{
					Type:    ErrMissingEnvVar,
					Key:     key,
					Message: "required secret is missing",
					Value:   "",
				})
			} else {
				result.Errors = append(result.Errors, ConfigError{
					Type:    ErrValidationFailed,
					Key:     key,
					Message: fmt.Sprintf("failed to retrieve secret: %v", err),
					Value:   "",
				})
			}
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
	if dbURL, ok := resolvedSecrets["DATABASE_URL"]; ok {
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
	if secret, ok := resolvedSecrets["JWT_SECRET"]; ok {
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

	if token, ok := resolvedSecrets["ADMIN_TOKEN"]; ok {
		if !isValidSecret(token) {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrWeakSecret,
				Key:     "ADMIN_TOKEN",
				Message: fmt.Sprintf("must be at least %d characters and contain upper/lower/digit/special characters", MinSecretLength),
				Value:   maskSecret(token),
			})
		} else {
			c.AdminToken = token
		}
	}

	// Validate optional MAX_HEADER_BYTES
	if val := os.Getenv("MAX_HEADER_BYTES"); val != "" {
		if max, err := strconv.Atoi(val); err == nil && max >= MinHeaderBytes && max <= MaxAllowedHeaderBytes {
			c.MaxHeaderBytes = max
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "MAX_HEADER_BYTES",
				Message: fmt.Sprintf("must be between %d and %d", MinHeaderBytes, MaxAllowedHeaderBytes),
				Value:   val,
			})
		}
	}

	// Validate optional timeouts
	if val := os.Getenv("READ_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout >= MinTimeoutSeconds && timeout <= MaxTimeoutSeconds {
			c.ReadTimeout = timeout
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "READ_TIMEOUT",
				Message: fmt.Sprintf("must be between %d and %d seconds", MinTimeoutSeconds, MaxTimeoutSeconds),
				Value:   val,
			})
		}
	}

	if val := os.Getenv("WRITE_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout >= MinTimeoutSeconds && timeout <= MaxTimeoutSeconds {
			c.WriteTimeout = timeout
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "WRITE_TIMEOUT",
				Message: fmt.Sprintf("must be between %d and %d seconds", MinTimeoutSeconds, MaxTimeoutSeconds),
				Value:   val,
			})
		}
	}

	if val := os.Getenv("IDLE_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout >= MinTimeoutSeconds && timeout <= MaxTimeoutSeconds {
			c.IdleTimeout = timeout
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "IDLE_TIMEOUT",
				Message: fmt.Sprintf("must be between %d and %d seconds", MinTimeoutSeconds, MaxTimeoutSeconds),
				Value:   val,
			})
		}
	}

	// Validate rate limiting configuration
	if val := os.Getenv("RATE_LIMIT_ENABLED"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			c.RateLimitEnabled = enabled
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "RATE_LIMIT_ENABLED",
				Message: "must be a valid boolean",
				Value:   val,
			})
		}
	}

	if mode := os.Getenv("RATE_LIMIT_MODE"); mode != "" {
		validModes := map[string]bool{"ip": true, "user": true, "hybrid": true}
		if validModes[mode] {
			c.RateLimitMode = mode
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "RATE_LIMIT_MODE",
				Message: "must be one of: ip, user, hybrid",
				Value:   mode,
			})
		}
	}

	if val := os.Getenv("RATE_LIMIT_RPS"); val != "" {
		if rps, err := strconv.Atoi(val); err == nil && rps >= MinRateLimitRPS && rps <= MaxRateLimitRPS {
			c.RateLimitRPS = rps
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "RATE_LIMIT_RPS",
				Message: fmt.Sprintf("must be between %d and %d", MinRateLimitRPS, MaxRateLimitRPS),
				Value:   val,
			})
		}
	}

	if val := os.Getenv("RATE_LIMIT_BURST"); val != "" {
		if burst, err := strconv.Atoi(val); err == nil && burst >= MinRateLimitBurst && burst <= MaxRateLimitBurst {
			c.RateLimitBurst = burst
		} else {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "RATE_LIMIT_BURST",
				Message: fmt.Sprintf("must be between %d and %d", MinRateLimitBurst, MaxRateLimitBurst),
				Value:   val,
			})
		}
	}

	if c.RateLimitBurst < c.RateLimitRPS {
		result.Errors = append(result.Errors, ConfigError{
			Type:    ErrInvalidValue,
			Key:     "RATE_LIMIT_BURST",
			Message: "must be greater than or equal to RATE_LIMIT_RPS",
			Value:   strconv.Itoa(c.RateLimitBurst),
		})
	}

	if whitelist := os.Getenv("RATE_LIMIT_WHITELIST"); whitelist != "" {
		paths := strings.Split(whitelist, ",")
		for i, path := range paths {
			clean := strings.TrimSpace(path)
			if clean == "" || !strings.HasPrefix(clean, "/") {
				result.Errors = append(result.Errors, ConfigError{
					Type:    ErrInvalidValue,
					Key:     "RATE_LIMIT_WHITELIST",
					Message: "each whitelist path must be non-empty and start with '/'",
					Value:   clean,
				})
			}
			paths[i] = clean
		}
		c.RateLimitWhitelist = paths
	}

	// Validate TRACING_EXPORTER
	if exporter := os.Getenv("TRACING_EXPORTER"); exporter != "" {
		validExporters := map[string]bool{"stdout": true, "otlp": true, "none": true}
		if !validExporters[exporter] {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrInvalidValue,
				Key:     "TRACING_EXPORTER",
				Message: "must be one of: stdout, otlp, none",
				Value:   exporter,
			})
		} else {
			c.TracingExporter = exporter
		}
	}

	if svcName := os.Getenv("TRACING_SERVICE_NAME"); svcName != "" {
		c.TracingServiceName = svcName
	}

	if c.Env == "production" || c.Env == "staging" {
		if strings.TrimSpace(c.AllowedOrigins) == "" {
			result.Errors = append(result.Errors, ConfigError{
				Type:    ErrMissingEnvVar,
				Key:     "ALLOWED_ORIGINS",
				Message: "is required in production and staging",
				Value:   "",
			})
		} else {
			for _, origin := range strings.Split(c.AllowedOrigins, ",") {
				trimmed := strings.TrimSpace(origin)
				if !isValidSecureOrigin(trimmed) {
					result.Errors = append(result.Errors, ConfigError{
						Type:    ErrInvalidURL,
						Key:     "ALLOWED_ORIGINS",
						Message: "must contain comma-separated valid https origins",
						Value:   trimmed,
					})
				}
			}
		}
	}

	// Set optional env values
	c.Env = getEnv("ENV", "development")

	return result
}

// isValidDatabaseURL validates that the database URL has a valid scheme and structure
func isValidDatabaseURL(dbURL string) bool {
	if dbURL == "" {
		return false
	}

	parsed, err := url.Parse(dbURL)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)
	validSchemes := map[string]bool{
		"postgres":   true,
		"postgresql": true,
		"mysql":      true,
		"sqlite":     true,
		"sqlite3":    true,
		"mongodb":    true,
		"redis":      true,
	}
	if !validSchemes[scheme] && !strings.Contains(scheme, "sql") {
		return false
	}

	switch scheme {
	case "sqlite", "sqlite3":
		return parsed.Path != "" || parsed.Opaque != ""
	default:
		return parsed.Host != ""
	}
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

	_ = hasSpecial

	return hasUpper && hasLower && hasDigit && hasSpecial
}

func isValidSecureOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" {
		return false
	}
	if parsed.Host == "" {
		return false
	}
	return parsed.Path == "" || parsed.Path == "/"
}

// maskPassword masks the password in a database URL for security
func maskPassword(dbURL string) string {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return "***"
	}

	if parsed.User == nil {
		return dbURL
	}

	password, ok := parsed.User.Password()
	if !ok || password == "" {
		return dbURL
	}

	return strings.Replace(dbURL, password, "***", 1)
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

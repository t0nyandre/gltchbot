package config

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Discord
	DiscordToken      string
	DiscordAppID      string
	DiscordDevGuildID string // empty in production → global commands

	// Bot presence
	BotStatus       string
	BotActivityType discordgo.ActivityType
	BotActivityText string

	// Database
	DBHost            string
	DBPort            int
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	DBMaxConns        int
	DBMinConns        int
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration

	// API
	APIPort               int
	APIKey                string // Deprecated: use APIKeys instead
	APIKeys               []string
	OldAPIKeys            []string
	RequestSizeLimitBytes int64
	APIRateLimitGlobal    float64
	APIRateLimitAuth      float64
	APIRateLimitUnauth    float64
	APIRateLimitBurst     float64

	// Audit logging
	AuditLogLevel string

	// Security headers
	SecurityHSTSMaxAge        int
	SecurityCSP               string
	SecurityPermissionsPolicy string

	// CORS
	CORSAllowedOrigins   string
	CORSAllowedMethods   string
	CORSAllowedHeaders   string
	CORSExposedHeaders   string
	CORSMaxAge           int
	CORSAllowCredentials bool
}

// DSN returns a PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

// Load reads configuration from environment variables.
// It attempts to load a .env file first (ignored if not present).
func Load() (*Config, error) {
	// Load .env if present; ignore error (file may not exist in production)
	_ = godotenv.Load()

	cfg := &Config{}

	// Discord
	token, err := requireEnv("DISCORD_TOKEN")
	if err != nil {
		return nil, err
	}
	cfg.DiscordToken = token

	appID, err := requireEnv("DISCORD_APP_ID")
	if err != nil {
		return nil, err
	}
	cfg.DiscordAppID = appID

	cfg.DiscordDevGuildID = os.Getenv("DISCORD_DEV_GUILD_ID")

	// Bot presence
	cfg.BotStatus = getEnvOrDefault("BOT_STATUS", "online")
	cfg.BotActivityType = parseActivityType(getEnvOrDefault("BOT_ACTIVITY_TYPE", "watching"))
	cfg.BotActivityText = getEnvOrDefault("BOT_ACTIVITY_TEXT", "over your channels")

	// Database
	cfg.DBHost = getEnvOrDefault("DB_HOST", "localhost")
	port, err := strconv.Atoi(getEnvOrDefault("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("DB_PORT must be between 1 and 65535, got %d", port)
	}
	cfg.DBPort = port
	cfg.DBUser = getEnvOrDefault("DB_USER", "gltchbot")
	password, err := requireEnv("DB_PASSWORD")
	if err != nil {
		return nil, err
	}
	cfg.DBPassword = password
	cfg.DBName = getEnvOrDefault("DB_NAME", "gltchbot")
	cfg.DBSSLMode = getEnvOrDefault("DB_SSLMODE", "disable")

	// Connection pool settings
	maxConns, err := strconv.Atoi(getEnvOrDefault("DB_MAX_CONNS", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONNS: %w", err)
	}
	if maxConns < 1 {
		return nil, fmt.Errorf("DB_MAX_CONNS must be at least 1, got %d", maxConns)
	}
	if maxConns > math.MaxInt32 {
		return nil, fmt.Errorf("DB_MAX_CONNS exceeds maximum allowed value %d, got %d", math.MaxInt32, maxConns)
	}
	cfg.DBMaxConns = maxConns

	minConns, err := strconv.Atoi(getEnvOrDefault("DB_MIN_CONNS", "2"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MIN_CONNS: %w", err)
	}
	if minConns < 0 {
		return nil, fmt.Errorf("DB_MIN_CONNS must be at least 0, got %d", minConns)
	}
	if minConns > math.MaxInt32 {
		return nil, fmt.Errorf("DB_MIN_CONNS exceeds maximum allowed value %d, got %d", math.MaxInt32, minConns)
	}
	if minConns > maxConns {
		return nil, fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", minConns, maxConns)
	}
	cfg.DBMinConns = minConns

	maxConnLifetime, err := time.ParseDuration(getEnvOrDefault("DB_MAX_CONN_LIFETIME", "1h"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONN_LIFETIME: %w", err)
	}
	cfg.DBMaxConnLifetime = maxConnLifetime

	maxConnIdleTime, err := time.ParseDuration(getEnvOrDefault("DB_MAX_CONN_IDLE_TIME", "30m"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONN_IDLE_TIME: %w", err)
	}
	cfg.DBMaxConnIdleTime = maxConnIdleTime

	// API
	apiPort, err := strconv.Atoi(getEnvOrDefault("API_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid API_PORT: %w", err)
	}
	if apiPort < 1 || apiPort > 65535 {
		return nil, fmt.Errorf("API_PORT must be between 1 and 65535, got %d", apiPort)
	}
	cfg.APIPort = apiPort
	apiKeyStr, err := requireEnv("API_KEY")
	if err != nil {
		return nil, err
	}
	cfg.APIKeys = parseCommaSeparatedKeys(apiKeyStr)
	if len(cfg.APIKeys) == 0 {
		return nil, fmt.Errorf("API_KEY must contain at least one valid key")
	}
	// Validate each API key
	for i, key := range cfg.APIKeys {
		if err := validateAPIKeyStrict(key); err != nil {
			return nil, fmt.Errorf("API_KEY entry %d invalid: %w", i+1, err)
		}
	}
	cfg.APIKey = cfg.APIKeys[0] // backward compatibility
	warnWeakKeys(cfg.APIKeys, "API_KEY")
	// Old API keys (optional)
	oldKeysStr := os.Getenv("OLD_API_KEYS")
	cfg.OldAPIKeys = parseCommaSeparatedKeys(oldKeysStr)
	// Validate old keys (same strictness)
	for i, key := range cfg.OldAPIKeys {
		if err := validateAPIKeyStrict(key); err != nil {
			return nil, fmt.Errorf("OLD_API_KEYS entry %d invalid: %w", i+1, err)
		}
	}
	warnWeakKeys(cfg.OldAPIKeys, "OLD_API_KEYS")

	// Request size limit
	sizeLimitStr := getEnvOrDefault("REQUEST_SIZE_LIMIT", "10MB")
	sizeLimitBytes, err := parseSizeBytes(sizeLimitStr)
	if err != nil {
		return nil, fmt.Errorf("invalid REQUEST_SIZE_LIMIT: %w", err)
	}
	cfg.RequestSizeLimitBytes = sizeLimitBytes

	// Rate limiting
	cfg.APIRateLimitGlobal = parseEnvFloat("API_RATE_LIMIT_GLOBAL", 100)
	cfg.APIRateLimitAuth = parseEnvFloat("API_RATE_LIMIT_AUTH", 50)
	cfg.APIRateLimitUnauth = parseEnvFloat("API_RATE_LIMIT_UNAUTH", 10)
	cfg.APIRateLimitBurst = parseEnvFloat("API_RATE_LIMIT_BURST", 2)

	// Audit logging
	cfg.AuditLogLevel = getEnvOrDefault("AUDIT_LOG_LEVEL", "info")

	// Security headers
	hstsMaxAge, err := strconv.Atoi(getEnvOrDefault("SECURITY_HSTS_MAX_AGE", "31536000"))
	if err != nil {
		return nil, fmt.Errorf("invalid SECURITY_HSTS_MAX_AGE: %w", err)
	}
	if hstsMaxAge < 0 {
		return nil, fmt.Errorf("SECURITY_HSTS_MAX_AGE must be non-negative, got %d", hstsMaxAge)
	}
	if hstsMaxAge > math.MaxInt32 {
		return nil, fmt.Errorf("SECURITY_HSTS_MAX_AGE exceeds maximum allowed value %d, got %d", math.MaxInt32, hstsMaxAge)
	}
	cfg.SecurityHSTSMaxAge = hstsMaxAge
	cfg.SecurityCSP = os.Getenv("SECURITY_CSP")
	cfg.SecurityPermissionsPolicy = os.Getenv("SECURITY_PERMISSIONS_POLICY")

	// CORS
	cfg.CORSAllowedOrigins = getEnvOrDefault("CORS_ALLOWED_ORIGINS", "*")
	cfg.CORSAllowedMethods = getEnvOrDefault("CORS_ALLOWED_METHODS", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	cfg.CORSAllowedHeaders = getEnvOrDefault("CORS_ALLOWED_HEADERS", "Content-Type, X-API-Key, Authorization")
	cfg.CORSExposedHeaders = os.Getenv("CORS_EXPOSED_HEADERS")
	maxAge, err := strconv.Atoi(getEnvOrDefault("CORS_MAX_AGE", "86400"))
	if err != nil {
		return nil, fmt.Errorf("invalid CORS_MAX_AGE: %w", err)
	}
	if maxAge < 0 {
		return nil, fmt.Errorf("CORS_MAX_AGE must be non-negative, got %d", maxAge)
	}
	if maxAge > math.MaxInt32 {
		return nil, fmt.Errorf("CORS_MAX_AGE exceeds maximum allowed value %d, got %d", math.MaxInt32, maxAge)
	}
	cfg.CORSMaxAge = maxAge
	allowCredentials := getEnvOrDefault("CORS_ALLOW_CREDENTIALS", "true")
	cfg.CORSAllowCredentials = allowCredentials == "true"

	return cfg, nil
}

// requireEnv returns the value of an env var or returns an error if not set.
func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// parseEnvFloat reads an environment variable and parses it as float64.
// If not set or invalid, returns defaultVal.
func parseEnvFloat(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return defaultVal
}

// parseActivityType maps human-readable strings to discordgo ActivityType constants.
func parseActivityType(s string) discordgo.ActivityType {
	switch s {
	case "playing":
		return discordgo.ActivityTypeGame // 0
	case "streaming":
		return discordgo.ActivityTypeStreaming // 1
	case "listening":
		return discordgo.ActivityTypeListening // 2
	case "watching":
		return discordgo.ActivityTypeWatching // 3
	case "competing":
		return discordgo.ActivityTypeCompeting // 5
	default:
		return discordgo.ActivityTypeWatching
	}
}

// parseSizeBytes parses human-readable size strings like "10MB", "100KB", "1GB" into bytes.
func parseSizeBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty size string")
	}

	// Find the numeric part
	var i int
	for i = 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			break
		}
	}
	if i == 0 {
		return 0, fmt.Errorf("no numeric prefix in %q", s)
	}
	numStr := s[:i]
	unit := strings.ToLower(strings.TrimSpace(s[i:]))

	// Parse numeric part
	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric part %q: %w", numStr, err)
	}

	// Multiply by unit
	switch unit {
	case "", "b", "bytes":
		// bytes
	case "k", "kb":
		val *= 1024
	case "m", "mb":
		val *= 1024 * 1024
	case "g", "gb":
		val *= 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit %q (supported: B, KB, MB, GB)", unit)
	}
	return val, nil
}

// parseCommaSeparatedKeys splits a comma-separated string into trimmed non-empty keys.
func parseCommaSeparatedKeys(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var keys []string
	for _, p := range parts {
		key := strings.TrimSpace(p)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// validateAPIKey checks if an API key meets security requirements.
// Returns true if valid, false with reason if invalid.
func validateAPIKey(key string) (bool, string) {
	const minLength = 32
	if len(key) < minLength {
		return false, fmt.Sprintf("key too short (minimum %d characters)", minLength)
	}
	// Check for weak/default keys
	weakKeys := []string{
		"change_this_to_a_secure_random_string",
		"secret",
		"password",
		"api_key",
		"test",
		"admin",
		"default",
		"123456",
	}
	for _, weak := range weakKeys {
		if strings.Contains(strings.ToLower(key), weak) {
			return false, fmt.Sprintf("key contains weak substring %q", weak)
		}
	}
	// Check entropy (simple): at least 3 character classes (upper, lower, digit, special)
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	for _, ch := range key {
		if ch >= 'A' && ch <= 'Z' {
			hasUpper = true
		} else if ch >= 'a' && ch <= 'z' {
			hasLower = true
		} else if ch >= '0' && ch <= '9' {
			hasDigit = true
		} else {
			hasSpecial = true
		}
	}
	classCount := 0
	if hasUpper {
		classCount++
	}
	if hasLower {
		classCount++
	}
	if hasDigit {
		classCount++
	}
	if hasSpecial {
		classCount++
	}
	if classCount < 2 {
		return false, "key lacks sufficient character variety (use upper/lower/digit/special)"
	}
	return true, ""
}

// validateAPIKeyStrict validates API key strength and returns an error if invalid.
func validateAPIKeyStrict(key string) error {
	const minLength = 32
	if len(key) < minLength {
		return fmt.Errorf("API key too short (minimum %d characters)", minLength)
	}
	// Reject exact weak keys (case-insensitive)
	weakKeys := []string{
		"change_this_to_a_secure_random_string",
		"secret",
		"password",
		"api_key",
		"test",
		"admin",
		"default",
		"123456",
	}
	lowerKey := strings.ToLower(key)
	for _, weak := range weakKeys {
		if strings.Contains(lowerKey, weak) {
			return fmt.Errorf("API key contains weak substring %q", weak)
		}
	}
	// Require at least 2 character classes
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	for _, ch := range key {
		if ch >= 'A' && ch <= 'Z' {
			hasUpper = true
		} else if ch >= 'a' && ch <= 'z' {
			hasLower = true
		} else if ch >= '0' && ch <= '9' {
			hasDigit = true
		} else {
			hasSpecial = true
		}
	}
	classCount := 0
	if hasUpper {
		classCount++
	}
	if hasLower {
		classCount++
	}
	if hasDigit {
		classCount++
	}
	if hasSpecial {
		classCount++
	}
	if classCount < 2 {
		return fmt.Errorf("API key lacks sufficient character variety (use upper/lower/digit/special)")
	}
	return nil
}

// warnWeakKeys logs warnings for any invalid or weak API keys.
func warnWeakKeys(keys []string, envVar string) {
	for i, key := range keys {
		if valid, reason := validateAPIKey(key); !valid {
			log.Printf("[WARN] API key %d from %s: %s", i+1, envVar, reason)
		}
	}
}

// GetAllAPIKeys returns a slice of all valid API keys (current + old) for authentication.
func (c *Config) GetAllAPIKeys() []string {
	var all []string
	all = append(all, c.APIKeys...)
	all = append(all, c.OldAPIKeys...)
	return all
}

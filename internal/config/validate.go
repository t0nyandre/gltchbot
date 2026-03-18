package config

import (
	"os"
	"strings"

	"github.com/t0nyandre/gltchbot/internal/logging"
)

// env returns the environment (production, development, staging).
// Defaults to "development" if GO_ENV or NODE_ENV are not set.
func env() string {
	// Prefer GO_ENV, fallback to NODE_ENV
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = os.Getenv("NODE_ENV")
	}
	if env == "" {
		env = "development"
	}
	return strings.ToLower(env)
}

// IsProduction returns true if the current environment is production.
func IsProduction() bool {
	return env() == "production"
}

// IsDevelopment returns true if the current environment is development.
func IsDevelopment() bool {
	return env() == "development"
}

// ValidateConfig performs security validation on the configuration and logs warnings.
// This should be called after config.Load() in main functions.
func ValidateConfig(cfg *Config) {
	env := env()
	allowed := []string{"production", "development", "staging"}
	isAllowed := false
	for _, a := range allowed {
		if env == a {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		logging.Warn("unknown environment", "env", env, "allowed", allowed)
	}

	isProd := env == "production"
	isDev := env == "development"

	if isProd {
		logging.Warn("running in production mode - security checks enabled")
	}

	// CORS wildcard in production
	if isProd && cfg.CORSAllowedOrigins == "*" {
		logging.Warn("CORS_ALLOWED_ORIGINS is set to '*' in production - this is insecure",
			"recommendation", "set to specific origins")
	}

	// Database SSL disabled in production
	if isProd && cfg.DBSSLMode == "disable" {
		logging.Warn("DB_SSLMODE is 'disable' in production - database connections are unencrypted",
			"recommendation", "use 'require' or 'verify-full'")
	}
	// Warn about insecure SSL modes in production
	if isProd && (cfg.DBSSLMode == "allow" || cfg.DBSSLMode == "prefer") {
		logging.Warn("DB_SSLMODE is insecure in production",
			"mode", cfg.DBSSLMode,
			"recommendation", "use 'require' or 'verify-full' for enforced encryption")
	}

	// Weak or default API keys (already validated in Load, but extra warning for production)
	if isProd {
		for i, key := range cfg.APIKeys {
			if len(key) < 32 {
				logging.Warn("API key may be weak (too short) in production",
					"key_index", i+1,
					"length", len(key),
					"recommendation", "use at least 32 characters")
			}
		}
	}

	// Check for default Discord token placeholder
	if strings.Contains(cfg.DiscordToken, "your_discord_bot_token_here") ||
		strings.Contains(cfg.DiscordToken, "changeme") ||
		len(cfg.DiscordToken) < 20 {
		logging.Warn("Discord token appears weak or placeholder",
			"recommendation", "use a valid bot token")
	}

	// Check for default Discord application ID placeholder
	if strings.Contains(cfg.DiscordAppID, "your_discord_application_id_here") ||
		len(cfg.DiscordAppID) < 5 {
		logging.Warn("Discord application ID appears to be a placeholder",
			"recommendation", "use a valid application ID")
	}

	// Check for weak database password
	if strings.Contains(cfg.DBPassword, "changeme") || len(cfg.DBPassword) < 8 {
		logging.Warn("Database password appears weak or default",
			"recommendation", "use a strong, random password")
	}

	// Discord permissions: ensure necessary intents are enabled (cannot validate via config,
	// but we can warn about missing dev guild ID in development)
	if isDev && cfg.DiscordDevGuildID == "" {
		logging.Warn("DISCORD_DEV_GUILD_ID is empty in development - commands will be registered globally",
			"recommendation", "set to a test guild ID for faster command updates")
	}
	if isProd && cfg.DiscordDevGuildID != "" {
		logging.Warn("DISCORD_DEV_GUILD_ID is set in production - commands will be registered only to the specified guild",
			"recommendation", "leave empty for global command registration")
	}

	// Security headers: warn if missing CSP in production (optional)
	if isProd && cfg.SecurityCSP == "" {
		logging.Warn("SECURITY_CSP is empty in production - consider setting a Content Security Policy",
			"recommendation", "see https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP")
	}

	// CORS with credentials and wildcard origin (insecure)
	if cfg.CORSAllowCredentials && cfg.CORSAllowedOrigins == "*" {
		logging.Warn("CORS_ALLOW_CREDENTIALS=true with wildcard origin '*' is insecure",
			"recommendation", "set specific origins or disable credentials")
	}

	// Audit log level too verbose in production (debug)
	if isProd && cfg.AuditLogLevel == "debug" {
		logging.Warn("AUDIT_LOG_LEVEL=debug in production may log sensitive data",
			"recommendation", "use 'info' or higher")
	}
}
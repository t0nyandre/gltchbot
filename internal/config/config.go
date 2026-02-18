package config

import (
	"fmt"
	"os"
	"strconv"

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
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// API
	APIPort int
	APIKey  string
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
	cfg.DiscordToken = requireEnv("DISCORD_TOKEN")
	cfg.DiscordAppID = requireEnv("DISCORD_APP_ID")
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
	cfg.DBPort = port
	cfg.DBUser = getEnvOrDefault("DB_USER", "gltchbot")
	cfg.DBPassword = requireEnv("DB_PASSWORD")
	cfg.DBName = getEnvOrDefault("DB_NAME", "gltchbot")
	cfg.DBSSLMode = getEnvOrDefault("DB_SSLMODE", "disable")

	// API
	apiPort, err := strconv.Atoi(getEnvOrDefault("API_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid API_PORT: %w", err)
	}
	cfg.APIPort = apiPort
	cfg.APIKey = requireEnv("API_KEY")

	return cfg, nil
}

// requireEnv returns the value of an env var or panics with a helpful message.
func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
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

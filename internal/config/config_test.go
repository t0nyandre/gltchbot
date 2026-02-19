package config

import (
	"os"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "returns env value when set",
			key:          "TEST_VAR_SET",
			envValue:     "hello",
			defaultValue: "default",
			expected:     "hello",
		},
		{
			name:         "returns default when env not set",
			key:          "TEST_VAR_UNSET",
			envValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			got := getEnvOrDefault(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("getEnvOrDefault(%q, %q) = %q; want %q", tt.key, tt.defaultValue, got, tt.expected)
			}
		})
	}
}

func TestParseActivityType(t *testing.T) {
	tests := []struct {
		input    string
		expected discordgo.ActivityType
	}{
		{"playing", discordgo.ActivityTypeGame},
		{"streaming", discordgo.ActivityTypeStreaming},
		{"listening", discordgo.ActivityTypeListening},
		{"watching", discordgo.ActivityTypeWatching},
		{"competing", discordgo.ActivityTypeCompeting},
		{"unknown", discordgo.ActivityTypeWatching}, // default fallback
		{"", discordgo.ActivityTypeWatching},        // empty = default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseActivityType(tt.input)
			if got != tt.expected {
				t.Errorf("parseActivityType(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     5432,
		DBUser:     "gltchbot",
		DBPassword: "secret",
		DBName:     "gltchbot",
		DBSSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=gltchbot password=secret dbname=gltchbot sslmode=disable"
	got := cfg.DSN()
	if got != expected {
		t.Errorf("DSN() = %q; want %q", got, expected)
	}
}

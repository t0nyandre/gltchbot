package reactionroles

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestParseEmoji(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unicode emoji",
			input:    "✅",
			expected: "✅",
		},
		{
			name:     "custom emoji with id",
			input:    ":custom_emoji:1234567890",
			expected: "custom_emoji:1234567890",
		},
		{
			name:     "custom emoji without id",
			input:    ":custom_emoji:",
			expected: "custom_emoji",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace trimmed",
			input:    "  ✅  ",
			expected: "✅",
		},
		{
			name:     "already formatted custom emoji",
			input:    "custom_emoji:1234567890",
			expected: "custom_emoji:1234567890",
		},
		{
			name:     "discord full format with angle brackets",
			input:    "<:custom_emoji:1234567890>",
			expected: "custom_emoji:1234567890",
		},
		{
			name:     "discord full format with trailing > in id",
			input:    "<:custom_emoji:941675399606317056>",
			expected: "custom_emoji:941675399606317056",
		},
		{
			name:     "animated emoji with angle brackets",
			input:    "<a:animated_emoji:1234567890>",
			expected: "animated_emoji:1234567890",
		},
		{
			name:     "discord format with extra spaces",
			input:    "  <:custom_emoji:1234567890>  ",
			expected: "custom_emoji:1234567890",
		},
		{
			name:     "discord format without colon prefix",
			input:    "<custom_emoji:1234567890>",
			expected: "custom_emoji:1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEmoji(tt.input)
			if got != tt.expected {
				t.Errorf("parseEmoji(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatEmojiForStorage(t *testing.T) {
	tests := []struct {
		name     string
		emoji    discordgo.Emoji
		expected string
	}{
		{
			name: "custom emoji with id",
			emoji: discordgo.Emoji{
				Name: "custom_emoji",
				ID:   "1234567890",
			},
			expected: "custom_emoji:1234567890",
		},
		{
			name: "unicode emoji",
			emoji: discordgo.Emoji{
				Name: "✅",
			},
			expected: "✅",
		},
		{
			name:     "empty emoji",
			emoji:    discordgo.Emoji{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEmojiForStorage(tt.emoji)
			if got != tt.expected {
				t.Errorf("formatEmojiForStorage() = %q; want %q", got, tt.expected)
			}
		})
	}
}

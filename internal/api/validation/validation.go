package validation

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Snowflake ID validation constants
const (
	minSnowflakeLength = 17
	maxSnowflakeLength = 19
)

var (
	// snowflakeRegex validates that the string consists only of digits
	snowflakeRegex = regexp.MustCompile(`^\d+$`)

	// emojiRegex validates custom Discord emoji format: <:name:id> or <a:name:id> (animated)
	emojiRegex = regexp.MustCompile(`^<a?:[a-zA-Z0-9_]+:\d+>$`)

	// moduleNameRegex validates module names (alphanumeric and underscores)
	moduleNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

// ValidateDiscordID validates a Discord snowflake ID.
// A snowflake ID is a numeric string of length 17-19 digits.
func ValidateDiscordID(id string) error {
	if id == "" {
		return errors.New("discord ID cannot be empty")
	}

	// Check length
	if len(id) < minSnowflakeLength || len(id) > maxSnowflakeLength {
		return fmt.Errorf("discord ID must be %d-%d digits, got %d", minSnowflakeLength, maxSnowflakeLength, len(id))
	}

	// Check numeric
	if !snowflakeRegex.MatchString(id) {
		return errors.New("discord ID must contain only digits")
	}

	return nil
}

// ValidateGuildID validates a guild (server) ID.
func ValidateGuildID(id string) error {
	return ValidateDiscordID(id)
}

// ValidateUserID validates a user ID.
func ValidateUserID(id string) error {
	return ValidateDiscordID(id)
}

// ValidateChannelID validates a channel ID.
func ValidateChannelID(id string) error {
	return ValidateDiscordID(id)
}

// ValidateRoleID validates a role ID.
func ValidateRoleID(id string) error {
	return ValidateDiscordID(id)
}

// ValidateMessageID validates a message ID.
func ValidateMessageID(id string) error {
	return ValidateDiscordID(id)
}

// ValidateRequiredString validates that a string field is not empty.
func ValidateRequiredString(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

// ValidateMaxLength validates that a string field does not exceed the maximum length.
func ValidateMaxLength(field, value string, max int) error {
	length := utf8.RuneCountInString(value)
	if length > max {
		return fmt.Errorf("%s exceeds maximum length of %d characters", field, max)
	}
	return nil
}

// ValidateMinLength validates that a string field meets the minimum length.
func ValidateMinLength(field, value string, min int) error {
	length := utf8.RuneCountInString(value)
	if length < min {
		return fmt.Errorf("%s must be at least %d characters", field, min)
	}
	return nil
}

// ValidateEmoji validates an emoji string.
// It accepts Unicode emoji (single character) or custom Discord emoji format:
// <:name:id> or <a:name:id> for animated emojis.
func ValidateEmoji(value string) error {
	if value == "" {
		return errors.New("emoji cannot be empty")
	}

	// Check if it's a custom Discord emoji
	if len(value) > 2 && value[0] == '<' && value[len(value)-1] == '>' {
		// Custom emoji format validation
		if !emojiRegex.MatchString(value) {
			return errors.New("invalid custom emoji format")
		}
		return nil
	}

	// Unicode emoji - ensure it's not empty and reasonable length
	// Discord allows up to 32 characters for custom emoji names, but Unicode emojis are typically 1-2 characters
	// We'll limit to 32 characters for safety
	if utf8.RuneCountInString(value) > 32 {
		return errors.New("emoji too long")
	}
	return nil
}

// ValidateModuleName validates a module name.
// Module names must be alphanumeric with underscores, 1-32 characters.
func ValidateModuleName(name string) error {
	if err := ValidateRequiredString("module name", name); err != nil {
		return err
	}
	if err := ValidateMinLength("module name", name, 1); err != nil {
		return err
	}
	if err := ValidateMaxLength("module name", name, 32); err != nil {
		return err
	}
	if !moduleNameRegex.MatchString(name) {
		return errors.New("module name can only contain letters, numbers, and underscores")
	}
	return nil
}

// ValidateBool validates that a boolean field is present (always valid).
// This is a placeholder for consistency with other validation functions.
func ValidateBool(field string, value bool) error {
	return nil
}

// ValidateInt validates that an integer field is within a range.
func ValidateInt(field string, value, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", field, min, max)
	}
	return nil
}

// ParseAndValidateDiscordID parses a string as a Discord ID and validates it.
// Returns the string if valid, otherwise an error.
func ParseAndValidateDiscordID(id string) (string, error) {
	if err := ValidateDiscordID(id); err != nil {
		return "", err
	}
	return id, nil
}
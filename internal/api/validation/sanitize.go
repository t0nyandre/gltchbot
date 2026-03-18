package validation

import (
	"html"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// SanitizeHTML escapes HTML entities in the given text, making it safe to embed
// within HTML content. This prevents XSS attacks when user input is rendered.
func SanitizeHTML(text string) string {
	return html.EscapeString(text)
}

// SanitizePath cleans a file path and prevents directory traversal attacks.
// It returns a cleaned path using filepath.Clean, and ensures the path does not
// contain ".." components or absolute paths. If the path attempts traversal,
// an empty string is returned.
func SanitizePath(path string) string {
	cleaned := filepath.Clean(path)
	// Check for directory traversal attempts
	if strings.Contains(cleaned, "..") || filepath.IsAbs(cleaned) {
		return ""
	}
	return cleaned
}

// IsValidDiscordID returns true if the given string is a valid Discord snowflake ID.
// A valid Discord ID is a numeric string of length 17-19 digits.
func IsValidDiscordID(id string) bool {
	return ValidateDiscordID(id) == nil
}

// uuidRegex matches UUID format (with or without hyphens).
// IsValidUUID returns true if the given string is a valid UUID.
// It accepts both hyphenated and non-hyphenated versions (lowercase or uppercase).
func IsValidUUID(id string) bool {
	return uuidRegex.MatchString(id)
}

// NormalizeString trims leading/trailing whitespace and normalizes Unicode
// to NFC form. It also collapses multiple whitespace characters into a single space.
func NormalizeString(s string) string {
	// Trim spaces
	s = strings.TrimSpace(s)
	// Normalize Unicode to NFC
	s = norm.NFC.String(s)
	// Collapse multiple whitespace into single space
	runes := []rune(s)
	var result []rune
	prevSpace := false
	for _, r := range runes {
		if unicode.IsSpace(r) {
			if !prevSpace {
				result = append(result, ' ')
				prevSpace = true
			}
		} else {
			result = append(result, r)
			prevSpace = false
		}
	}
	return string(result)
}

// SanitizeForLog removes newlines, control characters, and limits length
// to prevent log injection attacks. It ensures the string is safe to include
// in log lines.
func SanitizeForLog(s string) string {
	// Limit length to prevent log flooding (max 1024 characters)
	const maxLogLength = 1024
	if utf8.RuneCountInString(s) > maxLogLength {
		s = string([]rune(s)[:maxLogLength]) + "..."
	}
	// Remove control characters and newlines
	var result strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(' ')
		} else if unicode.IsControl(r) {
			// Skip other control characters
			continue
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// SanitizeLogDetails sanitizes all string values in a details map for safe logging.
// It returns a new map with sanitized strings (other types are left unchanged).
func SanitizeLogDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	sanitized := make(map[string]any, len(details))
	for k, v := range details {
		switch val := v.(type) {
		case string:
			sanitized[k] = SanitizeForLog(val)
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}
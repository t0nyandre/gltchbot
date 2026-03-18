package validation

import (
	"testing"
)

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "angle brackets",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "ampersand",
			input:    "A & B & C",
			expected: "A &amp; B &amp; C",
		},
		{
			name:     "double quotes",
			input:    `"quoted"`,
			expected: "&#34;quoted&#34;",
		},
		{
			name:     "single quotes",
			input:    "'single'",
			expected: "&#39;single&#39;",
		},
		{
			name:     "mixed",
			input:    `<a href="javascript:alert('xss')">click</a>`,
			expected: "&lt;a href=&#34;javascript:alert(&#39;xss&#39;)&#34;&gt;click&lt;/a&gt;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeHTML(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeHTML(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean path",
			input:    "foo/bar",
			expected: "foo/bar",
		},
		{
			name:     "with dot dot",
			input:    "foo/../bar",
			expected: "",
		},
		{
			name:     "absolute path",
			input:    "/etc/passwd",
			expected: "",
		},
		{
			name:     "multiple dot dots",
			input:    "../../../etc/passwd",
			expected: "",
		},
		{
			name:     "dot dot inside",
			input:    "foo/../..",
			expected: "",
		},
		{
			name:     "clean with dot",
			input:    "./foo/bar",
			expected: "foo/bar",
		},
		{
			name:     "trailing slash",
			input:    "foo/bar/",
			expected: "foo/bar",
		},
		{
			name:     "empty",
			input:    "",
			expected: ".",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizePath(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidDiscordID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid 17 digits", "12345678901234567", true},
		{"valid 18 digits", "123456789012345678", true},
		{"valid 19 digits", "1234567890123456789", true},
		{"too short", "1234567890123456", false},
		{"too long", "12345678901234567890", false},
		{"non digits", "12345678901234567a", false},
		{"empty", "", false},
		{"with spaces", " 12345678901234567 ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidDiscordID(tt.input)
			if got != tt.expected {
				t.Errorf("IsValidDiscordID(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"hyphenated lowercase", "123e4567-e89b-12d3-a456-426614174000", true},
		{"hyphenated uppercase", "123E4567-E89B-12D3-A456-426614174000", true},
		{"no hyphens lowercase", "123e4567e89b12d3a456426614174000", true},
		{"no hyphens uppercase", "123E4567E89B12D3A456426614174000", true},
		{"mixed case", "123e4567-E89b-12d3-A456-426614174000", true},
		{"invalid chars", "123e4567-e89b-12d3-a456-42661417400g", false},
		{"wrong length", "123e4567-e89b-12d3-a456-42661417400", false},
		{"empty", "", false},
		{"with braces", "{123e4567-e89b-12d3-a456-426614174000}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidUUID(tt.input)
			if got != tt.expected {
				t.Errorf("IsValidUUID(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "trim spaces",
			input:    "  hello  ",
			expected: "hello",
		},
		{
			name:     "multiple spaces",
			input:    "hello   world",
			expected: "hello world",
		},
		{
			name:     "tabs and newlines",
			input:    "hello\t\n\rworld",
			expected: "hello world",
		},
		{
			name:     "unicode normalization",
			input:    "caf\u00e9", // precomposed é
			expected: "caf\u00e9",
		},
		{
			name:     "unicode combining marks",
			input:    "cafe\u0301", // e + combining acute accent
			expected: "caf\u00e9",  // should normalize to precomposed
		},
		{
			name:     "mixed whitespace",
			input:    "  hello \t\n world  ",
			expected: "hello world",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeString(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "newline",
			input:    "Hello\nworld",
			expected: "Hello world",
		},
		{
			name:     "carriage return",
			input:    "Hello\rworld",
			expected: "Hello world",
		},
		{
			name:     "tab",
			input:    "Hello\tworld",
			expected: "Hello world",
		},
		{
			name:     "control characters",
			input:    "Hello\x00world\x1b",
			expected: "Helloworld",
		},
		{
			name:     "long string",
			input:    string(make([]rune, 2000)),
			expected: "...",
		},
		{
			name:     "mixed",
			input:    "Hello\n\r\t\x00world",
			expected: "Hello   world",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeForLog(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

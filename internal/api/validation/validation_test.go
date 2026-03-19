package validation

import (
	"testing"
)

func TestValidateDiscordID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid 17 digits", "12345678901234567", false},
		{"valid 18 digits", "123456789012345678", false},
		{"valid 19 digits", "1234567890123456789", false},
		{"too short", "1234567890123456", true},
		{"too long", "12345678901234567890", true},
		{"non digits", "12345678901234567a", true},
		{"empty", "", true},
		{"leading zeros", "012345678901234567", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDiscordID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDiscordID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"hyphenated lowercase", "123e4567-e89b-12d3-a456-426614174000", false},
		{"hyphenated uppercase", "123E4567-E89B-12D3-A456-426614174000", false},
		{"no hyphens lowercase", "123e4567e89b12d3a456426614174000", false},
		{"no hyphens uppercase", "123E4567E89B12D3A456426614174000", false},
		{"mixed case", "123e4567-E89b-12d3-A456-426614174000", false},
		{"invalid chars", "123e4567-e89b-12d3-a456-42661417400g", true},
		{"wrong length", "123e4567-e89b-12d3-a456-42661417400", true},
		{"empty", "", true},
		{"with braces", "{123e4567-e89b-12d3-a456-426614174000}", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRequiredString(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		wantErr bool
	}{
		{"non-empty", "name", "hello", false},
		{"empty", "name", "", true},
		{"whitespace only", "name", "   ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequiredString(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequiredString(%q, %q) error = %v, wantErr %v", tt.field, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateModuleName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "autorole", false},
		{"with underscore", "reaction_roles", false},
		{"with numbers", "jointocreate2", false},
		{"empty", "", true},
		{"too long", "abcdefghijklmnopqrstuvwxyzabcdef", true},
		{"invalid chars", "module-name", true},
		{"space", "module name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModuleName(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModuleName(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmoji(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"unicode emoji", "🔥", false},
		{"custom emoji", "<:cool:123456789012345678>", false},
		{"animated emoji", "<a:dance:123456789012345678>", false},
		{"empty", "", true},
		{"invalid custom format", "<:cool:123>", true},
		{"too long unicode", "🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmoji(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmoji(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

package tags

import (
	"testing"
)

func TestIsValidColor(t *testing.T) {
	tests := []struct {
		color    string
		expected bool
	}{
		{"slate", true},
		{"gray", true},
		{"red", true},
		{"orange", true},
		{"amber", true},
		{"yellow", true},
		{"lime", true},
		{"green", true},
		{"emerald", true},
		{"teal", true},
		{"cyan", true},
		{"sky", true},
		{"blue", true},
		{"indigo", true},
		{"violet", true},
		{"purple", true},
		{"pink", true},
		{"invalid", false},
		{"", false},
		{"SLATE", false},
	}

	for _, tt := range tests {
		if got := isValidColor(tt.color); got != tt.expected {
			t.Errorf("isValidColor(%q) = %v; want %v", tt.color, got, tt.expected)
		}
	}
}

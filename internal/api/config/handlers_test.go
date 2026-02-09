package config

import (
	"reflect"
	"testing"
)

func TestCastConfigValue(t *testing.T) {
	tests := []struct {
		val      string
		dataType string
		want     any
	}{
		{"hello", "string", "hello"},
		{"true", "boolean", true},
		{"false", "boolean", false},
		{"invalid", "boolean", false},
		{"123.45", "number", 123.45},
		{"invalid", "number", 0.0},
		{"anything", "unknown", "anything"},
	}

	for _, tt := range tests {
		got := castConfigValue(tt.val, tt.dataType)
		if got != tt.want {
			t.Errorf("castConfigValue(%q, %q) = %v, want %v", tt.val, tt.dataType, got, tt.want)
		}
	}
}

func TestNestConfig(t *testing.T) {
	flat := map[string]any{
		"branding.appName":        "Gotodo",
		"branding.appLogoInitial": "G",
		"auth.loginTitle":         "Welcome",
		"ui.sidebar.width":        250,
		"flat":                    "value",
	}

	want := map[string]any{
		"branding": map[string]any{
			"appName":        "Gotodo",
			"appLogoInitial": "G",
		},
		"auth": map[string]any{
			"loginTitle": "Welcome",
		},
		"ui": map[string]any{
			"sidebar": map[string]any{
				"width": 250,
			},
		},
		"flat": "value",
	}

	got := NestConfig(flat)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NestConfig() = %v, want %v", got, want)
	}
}

func TestNestConfig_Conflict(t *testing.T) {
	flat := map[string]any{
		"a":   "value",
		"a.b": "subvalue",
	}

	// The current implementation: the first one wins or overwrite depending on order.
	// Map iteration is random, so it's not deterministic.
	// But NestConfig should at least not crash.
	_ = NestConfig(flat)
}

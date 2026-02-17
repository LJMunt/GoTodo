package logging

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type MockLevelSource struct {
	level string
}

func (m *MockLevelSource) GetLogLevel(ctx context.Context) (string, error) {
	return m.level, nil
}

func TestLevelRefresher(t *testing.T) {
	logger := Init()
	src := &MockLevelSource{level: "debug"}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	StartLevelRefresher(ctx, logger, src, 10*time.Millisecond)

	// Initial refresh should happen immediately
	if GetLevel() != zerolog.DebugLevel {
		t.Errorf("expected level debug after initial refresh, got %v", GetLevel())
	}

	src.level = "error"
	// Wait for ticker refresh
	time.Sleep(50 * time.Millisecond)

	if GetLevel() != zerolog.ErrorLevel {
		t.Errorf("expected level error after ticker refresh, got %v", GetLevel())
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"unknown", zerolog.InfoLevel},
	}

	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.expected {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

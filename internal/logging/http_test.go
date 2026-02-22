package logging

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		expectedLevel string
	}{
		{
			name:          "regular_request_info",
			method:        "GET",
			path:          "/api/v1/tasks",
			expectedLevel: "info",
		},
		{
			name:          "health_request_debug",
			method:        "GET",
			path:          "/api/v1/health",
			expectedLevel: "debug",
		},
		{
			name:          "health_post_request_info",
			method:        "POST",
			path:          "/api/v1/health",
			expectedLevel: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

			handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			logOutput := buf.String()
			if logOutput == "" {
				t.Fatal("expected log output, got empty string")
			}

			if !strings.Contains(logOutput, `"level":"`+tt.expectedLevel+`"`) {
				t.Errorf("expected log level %q, got output: %s", tt.expectedLevel, logOutput)
			}
		})
	}
}

func TestRequestLogger_WithRouter(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	r := chi.NewRouter()
	r.Use(RequestLogger(logger))
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"level":"debug"`) {
		t.Errorf("expected debug level for /api/v1/health via router, got: %s", logOutput)
	}
}

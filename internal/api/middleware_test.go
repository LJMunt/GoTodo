package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBodyLimitByPath(t *testing.T) {
	handler := BodyLimitByPath(10, 20, "/api/v1/admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("default limit exceeded", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(bytes.Repeat([]byte("a"), 11)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d", rec.Code)
		}
	})

	t.Run("admin limit allows larger body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/config/values", bytes.NewReader(bytes.Repeat([]byte("a"), 15)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("admin limit exceeded", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/config/values", bytes.NewReader(bytes.Repeat([]byte("a"), 25)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d", rec.Code)
		}
	})
}

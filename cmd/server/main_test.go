package main

import (
	"GoToDo/internal/api"
	"GoToDo/internal/app"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {
	// We pass nil for DB since we want to see if we can at least test non-DB endpoints
	// or if the router can be initialized.
	deps := app.Deps{DB: nil}
	r := api.NewRouter(deps)

	ts := httptest.NewServer(r)
	defer ts.Close()

	t.Run("Health check", func(t *testing.T) {
		res, err := http.Get(ts.URL + "/api/v1/health")
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			t.Errorf("expected status OK, got %v", res.Status)
		}
	})

	t.Run("Ready check (no DB)", func(t *testing.T) {
		// This should fail because DB is nil
		res, err := http.Get(ts.URL + "/api/v1/ready")
		if err != nil {
			t.Fatal(err)
		}
		// Based on updated ReadyHandler, if db is nil it returns 503.
		if res.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected 503 Service Unavailable, got %v", res.Status)
		}
	})

	t.Run("Protected route (no auth)", func(t *testing.T) {
		res, err := http.Get(ts.URL + "/api/v1/projects")
		if err != nil {
			t.Fatal(err)
		}
		// Expect 401 Unauthorized because we didn't provide a token
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %v", res.Status)
		}
	})
}

package auth

import (
	"GoToDo/internal/db"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMiddleware(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping middleware integration tests")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect to DB: %v", err)
	}
	defer pool.Close()

	// Setup: create a test user
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	var userID int64
	err = pool.QueryRow(ctx, "INSERT INTO users (email, password_hash, is_active, is_admin) VALUES ($1, $2, $3, $4) ON CONFLICT (email) DO UPDATE SET is_active=true, is_admin=false RETURNING id",
		"test@example.com", "hash", true, false).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to setup test user: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM users WHERE id=$1", userID)

	var adminID int64
	err = pool.QueryRow(ctx, "INSERT INTO users (email, password_hash, is_active, is_admin) VALUES ($1, $2, $3, $4) ON CONFLICT (email) DO UPDATE SET is_active=true, is_admin=true RETURNING id",
		"admin@example.com", "hash", true, true).Scan(&adminID)
	if err != nil {
		t.Fatalf("failed to setup admin user: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM users WHERE id=$1", adminID)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := FromContext(r.Context())
		if !ok {
			t.Error("user not found in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("RequireAuth - Valid Token", func(t *testing.T) {
		token, _ := SignToken(userID)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		RequireAuth(pool)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("RequireAuth - Missing Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		RequireAuth(pool)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("RequireAuth - Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid")
		rr := httptest.NewRecorder()

		RequireAuth(pool)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("RequireAdmin - Admin User", func(t *testing.T) {
		token, _ := SignToken(adminID)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler := RequireAuth(pool)(RequireAdmin(nextHandler))
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("RequireAdmin - Regular User", func(t *testing.T) {
		token, _ := SignToken(userID)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler := RequireAuth(pool)(RequireAdmin(nextHandler))
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})
}

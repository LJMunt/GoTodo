package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeMiddlewareDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db fakeMiddlewareDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func TestMiddleware_Fake(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	userID := int64(1)
	adminID := int64(2)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	db := fakeMiddlewareDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "FROM users") {
				uid := args[0].(int64)
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*bool) = (uid == adminID) // isAdmin
						*dest[1].(*bool) = true             // isActive
						return nil
					},
				}
			}
			if strings.Contains(sql, "FROM config_keys") {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*bool) = true // readOnly
						return nil
					},
				}
			}
			return fakeRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	t.Run("RequireAuth - Valid Token", func(t *testing.T) {
		token, _ := SignToken(userID)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		RequireAuth(db)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("RequireAuth - Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid")
		rr := httptest.NewRecorder()

		RequireAuth(db)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("RequireAdmin - Admin User", func(t *testing.T) {
		token, _ := SignToken(adminID)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler := RequireAuth(db)(RequireAdmin(nextHandler))
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

		handler := RequireAuth(db)(RequireAdmin(nextHandler))
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})

	t.Run("ReadOnly - Block regular user write", func(t *testing.T) {
		token, _ := SignToken(userID)
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler := RequireAuth(db)(ReadOnly(db)(nextHandler))
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rr.Code)
		}
	})

	t.Run("ReadOnly - Allow admin user write", func(t *testing.T) {
		token, _ := SignToken(adminID)
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler := RequireAuth(db)(ReadOnly(db)(nextHandler))
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("ReadOnly - Allow regular user GET", func(t *testing.T) {
		token, _ := SignToken(userID)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler := RequireAuth(db)(ReadOnly(db)(nextHandler))
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("ReadOnly - Allow login bypass", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		rr := httptest.NewRecorder()

		// Note: we don't need RequireAuth here because login doesn't have it yet
		ReadOnly(db)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("ReadOnly - Allow admin path bypass", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/any", nil)
		rr := httptest.NewRecorder()

		ReadOnly(db)(nextHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

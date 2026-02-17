package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeAdminDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (db fakeAdminDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func (db fakeAdminDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if db.execFn != nil {
		return db.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (db fakeAdminDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if db.queryFn != nil {
		return db.queryFn(ctx, sql, args...)
	}
	return nil, nil
}

func TestGetDatabaseMetricsHandler(t *testing.T) {
	db := fakeAdminDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if sql == "SELECT pg_database_size(current_database())" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*int64) = 1024 * 1024 * 10 // 10 MB
						return nil
					},
				}
			}
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 5     // numbackends
					*dest[1].(*int64) = 0   // deadlocks
					*dest[2].(*int64) = 100 // blks_read
					*dest[3].(*int64) = 900 // blks_hit
					return nil
				},
			}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	rec := httptest.NewRecorder()

	GetDatabaseMetricsHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp DatabaseMetricsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.DatabaseSize != "10.00 MB" {
		t.Errorf("expected 10.00 MB, got %s", resp.DatabaseSize)
	}
	if resp.Connections != 5 {
		t.Errorf("expected 5 connections, got %d", resp.Connections)
	}
	if resp.CacheHitRatio != 90.0 {
		t.Errorf("expected 90.0 cache hit ratio, got %f", resp.CacheHitRatio)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, tt := range tests {
		if got := formatBytes(tt.bytes); got != tt.want {
			t.Errorf("formatBytes(%d) = %v, want %v", tt.bytes, got, tt.want)
		}
	}
}

func TestEmailVerificationHandlers(t *testing.T) {
	verifiedAt := time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC)

	t.Run("GetEmailVerification", func(t *testing.T) {
		db := fakeAdminDB{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(**time.Time) = &verifiedAt
						return nil
					},
				}
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/email-verification", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		GetUserEmailVerificationHandler(db).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		gotStr, ok := resp["email_verified_at"].(string)
		if !ok {
			t.Fatalf("expected string for email_verified_at, got %T", resp["email_verified_at"])
		}
		gotTime, _ := time.Parse(time.RFC3339, gotStr)
		if !gotTime.Equal(verifiedAt) {
			t.Errorf("expected %v, got %v", verifiedAt, gotTime)
		}
	})

	t.Run("VerifyEmail", func(t *testing.T) {
		db := fakeAdminDB{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/verify-email", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		VerifyUserEmailHandler(db).ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}
	})

	t.Run("UnverifyEmail", func(t *testing.T) {
		db := fakeAdminDB{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/unverify-email", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		UnverifyUserEmailHandler(db).ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}
	})
}

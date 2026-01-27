package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeAdminDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db fakeAdminDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
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

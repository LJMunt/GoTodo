package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeRows struct {
	pgx.Rows
	data []map[string]any
	idx  int
}

func (r *fakeRows) Next() bool {
	return r.idx < len(r.data)
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.data[r.idx]
	*dest[0].(*string) = row["key"].(string)
	*dest[1].(*string) = row["value_json"].(string)
	r.idx++
	return nil
}

func (r *fakeRows) Close() {}

type fakeConfigDB struct {
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (db fakeConfigDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.queryFn(ctx, sql, args...)
}

func TestGetConfigStatusHandler(t *testing.T) {
	db := fakeConfigDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{
				data: []map[string]any{
					{"key": "auth.allowSignup", "value_json": "false"},
					{"key": "auth.requireEmailVerification", "value_json": "true"},
					{"key": "instance.readOnly", "value_json": "false"},
				},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/status", nil)
	rec := httptest.NewRecorder()

	GetConfigStatusHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp ConfigStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Auth.AllowSignup != false {
		t.Errorf("expected allowSignup false, got %v", resp.Auth.AllowSignup)
	}
	if resp.Auth.RequireEmailVerification != true {
		t.Errorf("expected requireEmailVerification true, got %v", resp.Auth.RequireEmailVerification)
	}
	if resp.Instance.ReadOnly != false {
		t.Errorf("expected readOnly false, got %v", resp.Instance.ReadOnly)
	}
}

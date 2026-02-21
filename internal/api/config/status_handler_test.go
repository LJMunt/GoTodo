package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGetConfigStatusHandler(t *testing.T) {
	db := mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				data: [][]any{
					{"auth.allowSignup", "false"},
					{"auth.requireEmailVerification", "true"},
					{"auth.allowReset", "true"},
					{"instance.readOnly", "false"},
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
	if resp.Auth.AllowReset != true {
		t.Errorf("expected allowReset true, got %v", resp.Auth.AllowReset)
	}
	if resp.Instance.ReadOnly != false {
		t.Errorf("expected readOnly false, got %v", resp.Instance.ReadOnly)
	}
}

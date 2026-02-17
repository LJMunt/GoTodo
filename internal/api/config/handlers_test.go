package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
)

type mockDB struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryFn(ctx, sql, args...)
}

func (m mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.queryRowFn(ctx, sql, args...)
}

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m mockRow) Scan(dest ...any) error {
	return m.scanFn(dest...)
}

type mockRows struct {
	pgx.Rows
	data [][]any
	idx  int
}

func (m *mockRows) Next() bool {
	return m.idx < len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	row := m.data[m.idx]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		case *any:
			*d = row[i]
		}
	}
	m.idx++
	return nil
}

func (m *mockRows) Close() {}

func (m *mockRows) Err() error { return nil }

func TestCastConfigValue(t *testing.T) {
	tests := []struct {
		val      string
		dataType string
		want     any
	}{
		{"hello", "string", "hello"},
		{"true", "boolean", true},
		{"false", "boolean", false},
		{"invalid", "boolean", false},
		{"123.45", "number", 123.45},
		{"invalid", "number", 0.0},
		{"anything", "unknown", "anything"},
	}

	for _, tt := range tests {
		got := castConfigValue(tt.val, tt.dataType)
		if got != tt.want {
			t.Errorf("castConfigValue(%q, %q) = %v, want %v", tt.val, tt.dataType, got, tt.want)
		}
	}
}

func TestNestConfig(t *testing.T) {
	flat := map[string]any{
		"branding.appName":        "Gotodo",
		"branding.appLogoInitial": "G",
		"auth.loginTitle":         "Welcome",
		"ui.sidebar.width":        250,
		"flat":                    "value",
	}

	want := map[string]any{
		"branding": map[string]any{
			"appName":        "Gotodo",
			"appLogoInitial": "G",
		},
		"auth": map[string]any{
			"loginTitle": "Welcome",
		},
		"ui": map[string]any{
			"sidebar": map[string]any{
				"width": 250,
			},
		},
		"flat": "value",
	}

	got := NestConfig(flat)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NestConfig() = %v, want %v", got, want)
	}
}

func TestNestConfig_Conflict(t *testing.T) {
	flat := map[string]any{
		"a":   "value",
		"a.b": "subvalue",
	}

	// The current implementation: the first one wins or overwrite depending on order.
	// Map iteration is random, so it's not deterministic.
	// But NestConfig should at least not crash.
	_ = NestConfig(flat)
}

func TestSecretMasking(t *testing.T) {
	t.Run("GetConfigHandler masks secrets", func(t *testing.T) {
		db := mockDB{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return mockRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*string) = "en"
						return nil
					},
				}
			},
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return &mockRows{
					data: [][]any{
						{"mail.smtp.password", "string", "encrypted-val", true},
						{"branding.appName", "string", "GoTodo", false},
					},
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
		rec := httptest.NewRecorder()

		GetConfigHandler(db).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		// Check nested structure
		mail := resp["mail"].(map[string]any)
		if mail["smtp"].(map[string]any)["password"] != "" {
			t.Errorf("expected masked secret, got %v", mail["smtp"].(map[string]any)["password"])
		}

		branding := resp["branding"].(map[string]any)
		if branding["appName"] != "GoTodo" {
			t.Errorf("expected GoTodo, got %v", branding["appName"])
		}
	})

	t.Run("GetConfigValuesHandler masks secrets", func(t *testing.T) {
		db := mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return &mockRows{
					data: [][]any{
						{"mail.smtp.password", "encrypted-val", true},
						{"auth.allowSignup", false, false},
					},
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config/values", nil)
		rec := httptest.NewRecorder()

		GetConfigValuesHandler(db).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp["mail.smtp.password"] != "" {
			t.Errorf("expected masked secret, got %v", resp["mail.smtp.password"])
		}
		if resp["auth.allowSignup"] != false {
			t.Errorf("expected false (bool), got %v (%T)", resp["auth.allowSignup"], resp["auth.allowSignup"])
		}
	})
}

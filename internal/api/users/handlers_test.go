package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeUserDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db fakeUserDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func TestMeHandler_Success(t *testing.T) {
	db := fakeUserDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = "user@example.com"
					*dest[1].(*bool) = true
					*dest[2].(*bool) = true
					return nil
				},
			}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 7, IsAdmin: true}))
	rec := httptest.NewRecorder()

	MeHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["email"] != "user@example.com" {
		t.Fatalf("unexpected email %v", resp["email"])
	}
	if resp["is_admin"] != true {
		t.Fatalf("unexpected is_admin %v", resp["is_admin"])
	}
	if resp["is_active"] != true {
		t.Fatalf("unexpected is_active %v", resp["is_active"])
	}
}

func TestMeHandler_NotFound(t *testing.T) {
	db := fakeUserDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(_ ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 7}))
	rec := httptest.NewRecorder()

	MeHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var resp apiError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "user not found" {
		t.Fatalf("unexpected error %q", resp.Error)
	}
}

package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeProjectDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db fakeProjectDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func TestCreateProjectHandler_Success(t *testing.T) {
	now := time.Now().UTC()
	desc := "Sample project"

	db := fakeProjectDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 10
					*dest[1].(*string) = "My Project"
					*dest[2].(**string) = &desc
					*dest[3].(*time.Time) = now
					*dest[4].(*time.Time) = now
					return nil
				},
			}
		},
	}

	body := `{"name":"My Project","description":"Sample project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1}))
	rec := httptest.NewRecorder()

	CreateProjectHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp ProjectResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != 10 || resp.Name != "My Project" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Description == nil || *resp.Description != desc {
		t.Fatalf("unexpected description: %#v", resp.Description)
	}
}

func TestGetProjectHandler_NotFound(t *testing.T) {
	db := fakeProjectDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(_ ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1}))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	GetProjectHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var resp apiError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "project not found" {
		t.Fatalf("unexpected error %q", resp.Error)
	}
}

package languages

import (
	"bytes"
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

type fakeRows struct {
	pgx.Rows
	langs []AdminLanguage
	index int
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.langs)
}

func (r *fakeRows) Scan(dest ...any) error {
	l := r.langs[r.index]
	r.index++
	*dest[0].(*string) = l.Code
	*dest[1].(*string) = l.Name
	if len(dest) > 2 {
		*dest[2].(*time.Time) = l.CreatedAt
		*dest[3].(*time.Time) = l.UpdatedAt
	}
	return nil
}

func (r *fakeRows) Close()     {}
func (r *fakeRows) Err() error { return nil }

type fakeDB struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (db *fakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.queryFn(ctx, sql, args...)
}

func (db *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func (db *fakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return db.execFn(ctx, sql, args...)
}

func TestListLanguagesHandler(t *testing.T) {
	db := &fakeDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{
				langs: []AdminLanguage{
					{Code: "de", Name: "German"},
					{Code: "en", Name: "English"},
				},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lang", nil)
	rec := httptest.NewRecorder()

	ListLanguagesHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []Language
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(resp))
	}
}

func TestAdminCreateLanguageHandler_Validation(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{"Empty code", `{"code":"","name":"English"}`, http.StatusBadRequest},
		{"Too long code", `{"code":"toolong","name":"English"}`, http.StatusBadRequest},
		{"Valid 2 letters", `{"code":"en","name":"English"}`, http.StatusCreated},
		{"Valid format xx-xx", `{"code":"en-gb","name":"English"}`, http.StatusCreated},
		{"Invalid format 3 letters", `{"code":"eng","name":"English"}`, http.StatusBadRequest},
		{"Invalid format 1 letter", `{"code":"e","name":"English"}`, http.StatusBadRequest},
		{"Invalid format numbers", `{"code":"12","name":"English"}`, http.StatusBadRequest},
		{"Invalid format special chars", `{"code":"e!","name":"English"}`, http.StatusBadRequest},
		{"Invalid format incomplete xx-", `{"code":"en-","name":"English"}`, http.StatusBadRequest},
		{"Invalid format too long hyphened", `{"code":"en-gb-us","name":"English"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &fakeDB{
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &fakeRow{
						scanFn: func(dest ...any) error {
							if len(dest) >= 4 {
								*dest[0].(*string) = args[0].(string)
								*dest[1].(*string) = args[1].(string)
								*dest[2].(*time.Time) = time.Now()
								*dest[3].(*time.Time) = time.Now()
							}
							return nil
						},
					}
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/lang", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			AdminCreateLanguageHandler(db).ServeHTTP(rec, req)
			if rec.Code != tt.code {
				t.Errorf("expected status %d, got %d", tt.code, rec.Code)
			}
		})
	}
}

func TestAdminDeleteLanguageHandler_Safety(t *testing.T) {
	// Mocking config lookup
	mockDefaultLang := "\"en\""
	queryCount := 0

	db := &fakeDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{
				scanFn: func(dest ...any) error {
					if queryCount == 0 {
						// First query is for default language
						*dest[0].(*string) = mockDefaultLang
					} else {
						// Second query is for language count
						*dest[0].(*int) = 2
					}
					queryCount++
					return nil
				},
			}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}

	// Try to delete 'en' (which is the default in this mock)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/lang/en", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "en")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	AdminDeleteLanguageHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status 409 for default language deletion, got %d", rec.Code)
	}

	// Reset query count and change mock to different default
	queryCount = 0
	mockDefaultLang = "\"fr\""

	// Try to delete 'en' when 'fr' is default - should be allowed now that we removed hardcoded 'en' protection
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/lang/en", nil)
	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("code", "en")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec = httptest.NewRecorder()

	AdminDeleteLanguageHandler(db).ServeHTTP(rec, req)

	if rec.Code == http.StatusConflict {
		t.Errorf("expected status other than 409 for non-default 'en' deletion, got %d", rec.Code)
	}
}

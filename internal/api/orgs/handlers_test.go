package orgs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authmw "GoToDo/internal/auth"
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

type fakeTx struct {
	db *fakeDB
}

func (tx *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *fakeTx) Commit(ctx context.Context) error          { return nil }
func (tx *fakeTx) Rollback(ctx context.Context) error        { return nil }
func (tx *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (tx *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (tx *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *fakeTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return tx.db.Exec(ctx, sql, arguments...)
}
func (tx *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return tx.db.Query(ctx, sql, args...)
}
func (tx *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.db.QueryRow(ctx, sql, args...)
}
func (tx *fakeTx) Conn() *pgx.Conn { return nil }

type fakeDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (db fakeDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return &fakeTx{db: &db}, nil
}

func (db fakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (db fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if db.queryRowFn == nil {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	return db.queryRowFn(ctx, sql, args...)
}

func (db fakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if db.execFn == nil {
		return pgconn.CommandTag{}, nil
	}
	return db.execFn(ctx, sql, args...)
}

func TestCreateOrganizationHandler(t *testing.T) {
	db := fakeDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "INSERT INTO orgs") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 1
					*dest[1].(*time.Time) = time.Now()
					*dest[2].(*time.Time) = time.Now()
					return nil
				}}
			}
			return fakeRow{scanFn: func(_ ...any) error { return nil }}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	user := authmw.User{ID: 1, PublicID: "user_1", WorkspaceID: 1, WorkspacePublicID: "ws_1"}
	ctx := authmw.WithUser(context.Background(), user)

	body := `{"name":"My Org"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs", strings.NewReader(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	CreateOrganizationHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp OrganizationResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "My Org" {
		t.Errorf("expected name 'My Org', got %q", resp.Name)
	}
	if resp.WorkspacePublicID == "" {
		t.Error("expected workspace_id to be set")
	}
}

func TestOrganizationsEnabled_Middleware(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected int
	}{
		{"Enabled", true, http.StatusOK},
		{"Disabled", false, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := fakeDB{
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return fakeRow{
						scanFn: func(dest ...any) error {
							*dest[0].(*bool) = tt.enabled
							return nil
						},
					}
				},
			}

			handler := OrganizationsEnabled(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expected {
				t.Errorf("expected status %d, got %d", tt.expected, rec.Code)
			}
		})
	}
}

func TestRequireOrgAdmin_Middleware(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		dbErr    error
		expected int
	}{
		{"Admin", "admin", nil, http.StatusOK},
		{"Member", "member", nil, http.StatusForbidden},
		{"NoMember", "", pgx.ErrNoRows, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := fakeDB{
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return fakeRow{
						scanFn: func(dest ...any) error {
							if tt.dbErr != nil {
								return tt.dbErr
							}
							*dest[0].(*string) = tt.role
							return nil
						},
					}
				},
			}

			user := authmw.User{ID: 1}
			ctx := authmw.WithUser(context.Background(), user)

			handler := RequireOrgAdmin(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/orgs/123", nil)
			req = req.WithContext(ctx)
			// Add chi URL param
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "123")
			req = httptest.NewRequest(http.MethodGet, "/orgs/123", nil)
			req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expected {
				t.Errorf("expected status %d, got %d", tt.expected, rec.Code)
			}
		})
	}
}

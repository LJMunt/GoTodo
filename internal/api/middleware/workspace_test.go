package middleware

import (
	"context"
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

type fakeDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if db.queryRowFn == nil {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	return db.queryRowFn(ctx, sql, args...)
}

func TestResolveWorkspace(t *testing.T) {
	tests := []struct {
		name              string
		workspacePublicID string
		enabled           bool
		isMember          bool
		expectedStatus    int
		expectedWSID      int64
	}{
		{"NoHeader", "", true, true, http.StatusOK, 1},
		{"ValidOrg", "org_123", true, true, http.StatusOK, 100},
		{"DisabledOrg", "org_123", false, true, http.StatusNotFound, 1},
		{"NoMemberOrg", "org_123", true, false, http.StatusForbidden, 1},
		{"NotFoundWS", "unknown", true, true, http.StatusNotFound, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := fakeDB{
				queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
					if sql == "SELECT id, type, user_id, org_id FROM workspaces WHERE public_id = $1" {
						if args[0] == "org_123" {
							return fakeRow{scanFn: func(dest ...any) error {
								*dest[0].(*int64) = 100
								*dest[1].(*string) = "org"
								*dest[2].(**int64) = nil
								orgID := int64(50)
								*dest[3].(**int64) = &orgID
								return nil
							}}
						}
						return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
					}
					if sql == "SELECT value_json FROM config_keys WHERE key = 'features.organizations'" {
						return fakeRow{scanFn: func(dest ...any) error {
							*dest[0].(*bool) = tt.enabled
							return nil
						}}
					}
					if sql == "SELECT EXISTS (\n\t\t\t\t\t\tSELECT 1 FROM org_members om\n\t\t\t\t\t\tJOIN orgs o ON o.id = om.org_id\n\t\t\t\t\t\tWHERE om.org_id = $1 AND om.user_id = $2 AND o.deleted_at IS NULL\n\t\t\t\t\t)" {
						return fakeRow{scanFn: func(dest ...any) error {
							*dest[0].(*bool) = tt.isMember
							return nil
						}}
					}
					return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
				},
			}

			user := authmw.User{ID: 1, WorkspaceID: 1, WorkspacePublicID: "user_1"}
			ctx := authmw.WithUser(context.Background(), user)

			handler := ResolveWorkspace(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, _ := authmw.FromContext(r.Context())
				if u.WorkspaceID != tt.expectedWSID {
					t.Errorf("expected workspace ID %d, got %d", tt.expectedWSID, u.WorkspaceID)
				}
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(ctx)
			if tt.workspacePublicID != "" {
				req.Header.Set("X-Workspace-ID", tt.workspacePublicID)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

package orgs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
	"GoToDo/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mockOrgService struct {
	service.OrgService
	CreateOrganizationFunc func(ctx context.Context, userID int64, name string) (*models.Organization, error)
	LeaveOrganizationFunc  func(ctx context.Context, userID, orgID int64) error
	GetMemberRoleFunc      func(ctx context.Context, userID, orgID int64) (models.OrgRole, error)
}

func (m *mockOrgService) CreateOrganization(ctx context.Context, userID int64, name string) (*models.Organization, error) {
	return m.CreateOrganizationFunc(ctx, userID, name)
}

func (m *mockOrgService) LeaveOrganization(ctx context.Context, userID, orgID int64) error {
	return m.LeaveOrganizationFunc(ctx, userID, orgID)
}

func (m *mockOrgService) GetMemberRole(ctx context.Context, userID, orgID int64) (models.OrgRole, error) {
	return m.GetMemberRoleFunc(ctx, userID, orgID)
}

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
	return db.queryRowFn(ctx, sql, args...)
}

func (db fakeDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (db fakeDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (db fakeDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return nil, nil
}

func TestCreateOrganizationHandler(t *testing.T) {
	os := &mockOrgService{
		CreateOrganizationFunc: func(ctx context.Context, userID int64, name string) (*models.Organization, error) {
			return &models.Organization{
				ID:        1,
				Name:      name,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	deps := app.Deps{OrgService: os}

	user := authmw.User{ID: 1, PublicID: "user_1", WorkspaceID: 1, WorkspacePublicID: "ws_1"}
	ctx := authmw.WithUser(context.Background(), user)

	body := `{"name":"My Org"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs", strings.NewReader(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	CreateOrganizationHandler(deps).ServeHTTP(rec, req)

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
}

func TestOrganizationsEnabled_Middleware(t *testing.T) {
	t.Run("Enabled", func(t *testing.T) {
		db := fakeDB{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*bool) = true
					return nil
				}}
			},
		}

		deps := app.Deps{DB: db}

		handler := OrganizationsEnabled(deps)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		db := fakeDB{
			queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*bool) = false
					return nil
				}}
			},
		}

		deps := app.Deps{DB: db}

		handler := OrganizationsEnabled(deps)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})
}

func TestRequireOrgAdmin_Middleware(t *testing.T) {
	t.Run("IsAdmin", func(t *testing.T) {
		os := &mockOrgService{
			GetMemberRoleFunc: func(ctx context.Context, userID, orgID int64) (models.OrgRole, error) {
				return models.RoleAdmin, nil
			},
		}

		deps := app.Deps{OrgService: os}

		user := authmw.User{ID: 1}
		ctx := authmw.WithUser(context.Background(), user)

		handler := RequireOrgAdmin(deps)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/orgs/123", nil)
		req = req.WithContext(ctx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestRequireOrgMember_Middleware(t *testing.T) {
	t.Run("IsMember", func(t *testing.T) {
		os := &mockOrgService{
			GetMemberRoleFunc: func(ctx context.Context, userID, orgID int64) (models.OrgRole, error) {
				return models.RoleMember, nil
			},
		}

		deps := app.Deps{OrgService: os}

		handler := RequireOrgMember(deps)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/orgs/123", nil)
		req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1}))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestLeaveOrganizationHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		os := &mockOrgService{
			LeaveOrganizationFunc: func(ctx context.Context, userID, orgID int64) error {
				return nil
			},
		}

		deps := app.Deps{OrgService: os}

		req := httptest.NewRequest(http.MethodPost, "/orgs/10/leave", nil)
		req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 5}))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "10")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		LeaveOrganizationHandler(deps).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Error", func(t *testing.T) {
		os := &mockOrgService{
			LeaveOrganizationFunc: func(ctx context.Context, userID, orgID int64) error {
				return service.ErrInvalidInput
			},
		}

		deps := app.Deps{OrgService: os}

		req := httptest.NewRequest(http.MethodPost, "/orgs/10/leave", nil)
		req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 5}))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "10")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		LeaveOrganizationHandler(deps).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

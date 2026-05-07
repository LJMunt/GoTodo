package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
	"GoToDo/internal/service"
)

type mockUserService struct {
	service.UserService
	GetUserMeFunc func(ctx context.Context, userID int64) (*models.User, *models.Workspace, error)
}

func (m *mockUserService) GetUserMe(ctx context.Context, userID int64) (*models.User, *models.Workspace, error) {
	return m.GetUserMeFunc(ctx, userID)
}

func TestMeHandler_Success(t *testing.T) {
	now := time.Now().UTC()
	us := &mockUserService{
		GetUserMeFunc: func(ctx context.Context, userID int64) (*models.User, *models.Workspace, error) {
			u := &models.User{
				ID:              userID,
				PublicID:        "ULID1234567890123456789012",
				Email:           "user@example.com",
				IsAdmin:         true,
				IsActive:        true,
				EmailVerifiedAt: &now,
				TOTPEnabled:     false,
				UITheme:         "system",
				Language:        "en",
			}
			ws := &models.Workspace{
				PublicID: "WS1234567890123456789012",
				Type:     models.WorkspaceTypeUser,
			}
			return u, ws, nil
		},
	}

	deps := app.Deps{UserService: us}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 7, IsAdmin: true}))
	rec := httptest.NewRecorder()

	MeHandler(deps).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp userMeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Email != "user@example.com" {
		t.Fatalf("unexpected email %v", resp.Email)
	}
	if resp.PublicID != "ULID1234567890123456789012" {
		t.Fatalf("unexpected public_id %v", resp.PublicID)
	}
	if resp.IsAdmin != true {
		t.Fatalf("unexpected is_admin %v", resp.IsAdmin)
	}
	if resp.IsActive != true {
		t.Fatalf("unexpected is_active %v", resp.IsActive)
	}
	if resp.MfaEnabled != false {
		t.Fatalf("unexpected mfa_enabled %v", resp.MfaEnabled)
	}
}

func TestMeHandler_NotFound(t *testing.T) {
	us := &mockUserService{
		GetUserMeFunc: func(ctx context.Context, userID int64) (*models.User, *models.Workspace, error) {
			return nil, nil, service.ErrNotFound
		},
	}

	deps := app.Deps{UserService: us}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 7}))
	rec := httptest.NewRecorder()

	MeHandler(deps).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var resp apiutil.APIError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != service.ErrNotFound.Error() {
		t.Fatalf("unexpected error %q", resp.Error)
	}
}

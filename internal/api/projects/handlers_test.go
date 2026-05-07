package projects

import (
	"bytes"
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

	"github.com/go-chi/chi/v5"
)

type mockProjectService struct {
	service.ProjectService
	CreateProjectFunc func(ctx context.Context, workspaceID int64, name string, description *string) (*models.Project, error)
	GetProjectFunc    func(ctx context.Context, workspaceID, projectID int64) (*models.Project, error)
}

func (m *mockProjectService) CreateProject(ctx context.Context, workspaceID int64, name string, description *string) (*models.Project, error) {
	return m.CreateProjectFunc(ctx, workspaceID, name, description)
}

func (m *mockProjectService) GetProject(ctx context.Context, workspaceID, projectID int64) (*models.Project, error) {
	return m.GetProjectFunc(ctx, workspaceID, projectID)
}

func TestCreateProjectHandler_Success(t *testing.T) {
	now := time.Now().UTC()
	desc := "Sample project"

	ps := &mockProjectService{
		CreateProjectFunc: func(ctx context.Context, workspaceID int64, name string, description *string) (*models.Project, error) {
			return &models.Project{
				ID:          10,
				Name:        name,
				Description: description,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	deps := app.Deps{ProjectService: ps}

	body := `{"name":"My Project","description":"Sample project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1}))
	rec := httptest.NewRecorder()

	CreateProjectHandler(deps).ServeHTTP(rec, req)

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
	ps := &mockProjectService{
		GetProjectFunc: func(ctx context.Context, workspaceID, projectID int64) (*models.Project, error) {
			return nil, service.ErrNotFound
		},
	}

	deps := app.Deps{ProjectService: ps}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/1", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1}))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	GetProjectHandler(deps).ServeHTTP(rec, req)

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

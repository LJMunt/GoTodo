package tasks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
	"GoToDo/internal/service"

	"github.com/go-chi/chi/v5"
)

type mockTaskService struct {
	service.TaskService
	GetTaskFunc func(ctx context.Context, workspaceID, taskID int64) (*models.Task, error)
}

func (m *mockTaskService) GetTask(ctx context.Context, workspaceID, taskID int64) (*models.Task, error) {
	return m.GetTaskFunc(ctx, workspaceID, taskID)
}

func TestGetTaskHandler_Success(t *testing.T) {
	ts := &mockTaskService{
		GetTaskFunc: func(ctx context.Context, workspaceID, taskID int64) (*models.Task, error) {
			return &models.Task{
				ID:          taskID,
				WorkspaceID: workspaceID,
				Title:       "Test Task",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}, nil
		},
	}

	deps := app.Deps{TaskService: ts}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/1", nil)
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1, WorkspaceID: 10, WorkspacePublicID: "WS10"}))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	GetTaskHandler(deps).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp TaskResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Title != "Test Task" {
		t.Errorf("unexpected title: %v", resp.Title)
	}
}

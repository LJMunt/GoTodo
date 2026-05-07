package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Mock DBTX
type mockDBTX struct{}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (m *mockDBTX) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockDBTX) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return &mockRow{}
}

type mockRow struct{}

func (m *mockRow) Scan(dest ...any) error {
	return nil
}

// Mock repositories
type mockTaskRepo struct {
	repository.TaskRepository
	GetByIDFunc func(ctx context.Context, db repository.DBTX, workspaceID, id int64) (*models.Task, error)
}

func (m *mockTaskRepo) GetByID(ctx context.Context, db repository.DBTX, workspaceID, id int64) (*models.Task, error) {
	return m.GetByIDFunc(ctx, db, workspaceID, id)
}
func (m *mockTaskRepo) IsVisible(ctx context.Context, db repository.DBTX, workspaceID, id int64) (bool, error) {
	return true, nil
}

func TestGetTask(t *testing.T) {
	tr := &mockTaskRepo{
		GetByIDFunc: func(ctx context.Context, db repository.DBTX, workspaceID, id int64) (*models.Task, error) {
			if id == 1 {
				return &models.Task{ID: 1, Title: "Test Task"}, nil
			}
			return nil, nil
		},
	}

	svc := &taskService{
		taskRepo: tr,
		pool:     nil, // Not used in this mock scenario if we don't call pool methods
	}

	ctx := context.Background()

	t.Run("Found", func(t *testing.T) {
		task, err := svc.GetTask(ctx, 100, 1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if task == nil || task.Title != "Test Task" {
			t.Errorf("expected task with title 'Test Task', got %v", task)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		task, err := svc.GetTask(ctx, 100, 2)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
		if task != nil {
			t.Errorf("expected nil task, got %v", task)
		}
	})
}

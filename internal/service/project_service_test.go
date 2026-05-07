package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"testing"
)

type mockProjectRepo struct {
	repository.ProjectRepository
	CreateFunc func(ctx context.Context, db repository.DBTX, project *models.Project) error
}

func (m *mockProjectRepo) Create(ctx context.Context, db repository.DBTX, project *models.Project) error {
	return m.CreateFunc(ctx, db, project)
}

func TestCreateProject(t *testing.T) {
	repo := &mockProjectRepo{
		CreateFunc: func(ctx context.Context, db repository.DBTX, p *models.Project) error {
			p.ID = 1
			return nil
		},
	}
	svc := NewProjectService(nil, repo)

	t.Run("Success", func(t *testing.T) {
		p, err := svc.CreateProject(context.Background(), 100, "New Project", nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.ID != 1 || p.Name != "New Project" {
			t.Errorf("unexpected project: %+v", p)
		}
	})

	t.Run("EmptyName", func(t *testing.T) {
		_, err := svc.CreateProject(context.Background(), 100, "", nil)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})
}

package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectService interface {
	CreateProject(ctx context.Context, workspaceID int64, name string, description *string) (*models.Project, error)
	GetProject(ctx context.Context, workspaceID, projectID int64) (*models.Project, error)
	UpdateProject(ctx context.Context, workspaceID, projectID int64, name *string, description *string) (*models.Project, error)
	DeleteProject(ctx context.Context, workspaceID, projectID int64) error
	ListProjects(ctx context.Context, workspaceID int64, includeDeleted bool) ([]*models.Project, error)
}

type projectService struct {
	pool        *pgxpool.Pool
	projectRepo repository.ProjectRepository
}

func NewProjectService(pool *pgxpool.Pool, projectRepo repository.ProjectRepository) ProjectService {
	return &projectService{
		pool:        pool,
		projectRepo: projectRepo,
	}
}

func (s *projectService) CreateProject(ctx context.Context, workspaceID int64, name string, description *string) (*models.Project, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	p := &models.Project{
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
	}

	if err := s.projectRepo.Create(ctx, s.pool, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *projectService) GetProject(ctx context.Context, workspaceID, projectID int64) (*models.Project, error) {
	p, err := s.projectRepo.GetByID(ctx, s.pool, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

func (s *projectService) UpdateProject(ctx context.Context, workspaceID, projectID int64, name *string, description *string) (*models.Project, error) {
	p, err := s.projectRepo.GetByID(ctx, s.pool, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}

	if name != nil {
		if *name == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrInvalidInput)
		}
		p.Name = *name
	}
	if description != nil {
		p.Description = description
	}

	if err := s.projectRepo.Update(ctx, s.pool, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *projectService) DeleteProject(ctx context.Context, workspaceID, projectID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ok, err := s.projectRepo.Exists(ctx, tx, workspaceID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}

	if err := s.projectRepo.Delete(ctx, tx, workspaceID, projectID); err != nil {
		return err
	}

	// Delete tasks under this project
	_, err = tx.Exec(ctx,
		`UPDATE tasks
		 SET deleted_at = now(), updated_at = now()
		 WHERE project_id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		projectID, workspaceID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *projectService) ListProjects(ctx context.Context, workspaceID int64, includeDeleted bool) ([]*models.Project, error) {
	return s.projectRepo.List(ctx, s.pool, workspaceID, includeDeleted)
}

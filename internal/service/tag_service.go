package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TagService interface {
	CreateTag(ctx context.Context, workspaceID int64, name, color string) (*models.Tag, error)
	GetTag(ctx context.Context, workspaceID, tagID int64) (*models.Tag, error)
	UpdateTag(ctx context.Context, workspaceID, tagID int64, name, color *string) (*models.Tag, error)
	DeleteTag(ctx context.Context, workspaceID, tagID int64) error
	ListTags(ctx context.Context, workspaceID int64, query string) ([]*models.Tag, error)
}

type tagService struct {
	pool    *pgxpool.Pool
	tagRepo repository.TagRepository
}

func NewTagService(pool *pgxpool.Pool, tagRepo repository.TagRepository) TagService {
	return &tagService{
		pool:    pool,
		tagRepo: tagRepo,
	}
}

func (s *tagService) CreateTag(ctx context.Context, workspaceID int64, name, color string) (*models.Tag, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	t := &models.Tag{
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       color,
	}

	if err := s.tagRepo.Create(ctx, s.pool, t); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *tagService) GetTag(ctx context.Context, workspaceID, tagID int64) (*models.Tag, error) {
	t, err := s.tagRepo.GetByID(ctx, s.pool, workspaceID, tagID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *tagService) UpdateTag(ctx context.Context, workspaceID, tagID int64, name, color *string) (*models.Tag, error) {
	t, err := s.tagRepo.GetByID(ctx, s.pool, workspaceID, tagID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrNotFound
	}

	if name != nil {
		if *name == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrInvalidInput)
		}
		t.Name = *name
	}
	if color != nil {
		t.Color = *color
	}

	if err := s.tagRepo.Update(ctx, s.pool, t); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *tagService) DeleteTag(ctx context.Context, workspaceID, tagID int64) error {
	t, err := s.tagRepo.GetByID(ctx, s.pool, workspaceID, tagID)
	if err != nil {
		return err
	}
	if t == nil {
		return ErrNotFound
	}

	return s.tagRepo.Delete(ctx, s.pool, workspaceID, tagID)
}

func (s *tagService) ListTags(ctx context.Context, workspaceID int64, query string) ([]*models.Tag, error) {
	return s.tagRepo.List(ctx, s.pool, workspaceID, query)
}

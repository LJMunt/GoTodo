package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UpdateUserParams struct {
	Email                *string
	UITheme              *string
	ShowCompletedDefault *bool
	Language             *string
}

type UserService interface {
	GetUserMe(ctx context.Context, userID int64) (*models.User, *models.Workspace, error)
	SearchUsers(ctx context.Context, query string) ([]*models.User, error)
	UpdateUser(ctx context.Context, userID int64, params UpdateUserParams) (*models.User, error)
	DeleteUser(ctx context.Context, userID int64) error
}

type userService struct {
	pool          *pgxpool.Pool
	userRepo      repository.UserRepository
	workspaceRepo repository.WorkspaceRepository
}

func NewUserService(pool *pgxpool.Pool, userRepo repository.UserRepository, workspaceRepo repository.WorkspaceRepository) UserService {
	return &userService{
		pool:          pool,
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
	}
}

func (s *userService) GetUserMe(ctx context.Context, userID int64) (*models.User, *models.Workspace, error) {
	user, err := s.userRepo.GetByID(ctx, s.pool, userID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrNotFound
	}

	ws, err := s.workspaceRepo.GetPersonalByUserID(ctx, s.pool, userID)
	if err != nil {
		return nil, nil, err
	}

	return user, ws, nil
}

func (s *userService) SearchUsers(ctx context.Context, query string) ([]*models.User, error) {
	if len(query) < 2 {
		return nil, nil
	}
	return s.userRepo.Search(ctx, s.pool, query, 10)
}

func (s *userService) UpdateUser(ctx context.Context, userID int64, p UpdateUserParams) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, s.pool, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	if p.Email != nil {
		user.Email = *p.Email
	}
	if p.UITheme != nil {
		user.UITheme = *p.UITheme
	}
	if p.ShowCompletedDefault != nil {
		user.ShowCompletedDefault = *p.ShowCompletedDefault
	}
	if p.Language != nil {
		user.Language = *p.Language
	}

	if err := s.userRepo.Update(ctx, s.pool, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) DeleteUser(ctx context.Context, userID int64) error {
	// In the original handler, it was a soft delete or hard delete?
	// Looking at the original code... it was a hard delete?
	// Actually it was a hard delete of users table which cascaded.
	return s.userRepo.Delete(ctx, s.pool, userID)
}

package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type WorkspaceRepository interface {
	GetByID(ctx context.Context, db DBTX, id int64) (*models.Workspace, error)
	GetPersonalByUserID(ctx context.Context, db DBTX, userID int64) (*models.Workspace, error)
}

type workspaceRepository struct{}

func NewWorkspaceRepository() WorkspaceRepository {
	return &workspaceRepository{}
}

func (r *workspaceRepository) GetByID(ctx context.Context, db DBTX, id int64) (*models.Workspace, error) {
	var w models.Workspace
	err := db.QueryRow(ctx,
		`SELECT id, public_id, type, user_id, org_id, created_at, updated_at
		 FROM workspaces WHERE id = $1`,
		id,
	).Scan(&w.ID, &w.PublicID, &w.Type, &w.UserID, &w.OrgID, &w.CreatedAt, &w.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (r *workspaceRepository) GetPersonalByUserID(ctx context.Context, db DBTX, userID int64) (*models.Workspace, error) {
	var w models.Workspace
	err := db.QueryRow(ctx,
		`SELECT id, public_id, type, user_id, org_id, created_at, updated_at
		 FROM workspaces WHERE user_id = $1 AND type = 'user'`,
		userID,
	).Scan(&w.ID, &w.PublicID, &w.Type, &w.UserID, &w.OrgID, &w.CreatedAt, &w.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

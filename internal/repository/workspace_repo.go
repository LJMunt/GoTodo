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
	ListForUser(ctx context.Context, db DBTX, userID int64) ([]*models.Workspace, error)
}

type workspaceRepository struct{}

func NewWorkspaceRepository() WorkspaceRepository {
	return &workspaceRepository{}
}

func (r *workspaceRepository) GetByID(ctx context.Context, db DBTX, id int64) (*models.Workspace, error) {
	var w models.Workspace
	err := db.QueryRow(ctx,
		`SELECT w.id, w.public_id, w.type, w.user_id, w.org_id, w.created_at, w.updated_at,
		        COALESCE(o.name, 'Personal') as name
		 FROM workspaces w
		 LEFT JOIN orgs o ON w.org_id = o.id
		 WHERE w.id = $1`,
		id,
	).Scan(&w.ID, &w.PublicID, &w.Type, &w.UserID, &w.OrgID, &w.CreatedAt, &w.UpdatedAt, &w.Name)

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
		`SELECT id, public_id, type, user_id, org_id, created_at, updated_at, 'Personal' as name
		 FROM workspaces WHERE user_id = $1 AND type = 'user'`,
		userID,
	).Scan(&w.ID, &w.PublicID, &w.Type, &w.UserID, &w.OrgID, &w.CreatedAt, &w.UpdatedAt, &w.Name)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (r *workspaceRepository) ListForUser(ctx context.Context, db DBTX, userID int64) ([]*models.Workspace, error) {
	rows, err := db.Query(ctx,
		`SELECT w.id, w.public_id, w.type, w.user_id, w.org_id, w.created_at, w.updated_at,
		        COALESCE(o.name, 'Personal') as name
		 FROM workspaces w
		 LEFT JOIN orgs o ON w.org_id = o.id
		 LEFT JOIN org_members om ON w.org_id = om.org_id
		 WHERE (w.type = 'user' AND w.user_id = $1)
		    OR (w.type = 'org' AND om.user_id = $1 AND o.deleted_at IS NULL)
		 ORDER BY (w.type = 'user') DESC, w.created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []*models.Workspace
	for rows.Next() {
		var w models.Workspace
		err := rows.Scan(&w.ID, &w.PublicID, &w.Type, &w.UserID, &w.OrgID, &w.CreatedAt, &w.UpdatedAt, &w.Name)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, &w)
	}
	return workspaces, nil
}

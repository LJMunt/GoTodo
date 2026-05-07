package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type ProjectRepository interface {
	GetByID(ctx context.Context, db DBTX, workspaceID, id int64) (*models.Project, error)
	Create(ctx context.Context, db DBTX, project *models.Project) error
	Update(ctx context.Context, db DBTX, project *models.Project) error
	Delete(ctx context.Context, db DBTX, workspaceID, id int64) error
	List(ctx context.Context, db DBTX, workspaceID int64, includeDeleted bool) ([]*models.Project, error)
	Exists(ctx context.Context, db DBTX, workspaceID, id int64) (bool, error)
}

type projectRepository struct{}

func NewProjectRepository() ProjectRepository {
	return &projectRepository{}
}

func (r *projectRepository) GetByID(ctx context.Context, db DBTX, workspaceID, id int64) (*models.Project, error) {
	var p models.Project
	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at, deleted_at
		 FROM projects
		 WHERE id=$1 AND workspace_id=$2 AND deleted_at IS NULL`,
		id, workspaceID,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *projectRepository) Create(ctx context.Context, db DBTX, p *models.Project) error {
	return db.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		p.WorkspaceID, p.Name, p.Description,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *projectRepository) Update(ctx context.Context, db DBTX, p *models.Project) error {
	_, err := db.Exec(ctx,
		`UPDATE projects SET name=$1, description=$2, updated_at=now()
		 WHERE id=$3 AND workspace_id=$4`,
		p.Name, p.Description, p.ID, p.WorkspaceID,
	)
	return err
}

func (r *projectRepository) Delete(ctx context.Context, db DBTX, workspaceID, id int64) error {
	_, err := db.Exec(ctx,
		"UPDATE projects SET deleted_at=now(), updated_at=now() WHERE id=$1 AND workspace_id=$2",
		id, workspaceID,
	)
	return err
}

func (r *projectRepository) List(ctx context.Context, db DBTX, workspaceID int64, includeDeleted bool) ([]*models.Project, error) {
	query := `SELECT id, workspace_id, name, description, created_at, updated_at, deleted_at
		 FROM projects
		 WHERE workspace_id=$1`
	if !includeDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY id"

	rows, err := db.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	return projects, nil
}

func (r *projectRepository) Exists(ctx context.Context, db DBTX, workspaceID, id int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM projects
		   WHERE id=$1 AND workspace_id=$2 AND deleted_at IS NULL
		 )`,
		id, workspaceID,
	).Scan(&ok)
	return ok, err
}

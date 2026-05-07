package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type TagRepository interface {
	ListByTaskID(ctx context.Context, db DBTX, workspaceID, taskID int64) ([]*models.Tag, error)
	AssignToTask(ctx context.Context, db DBTX, workspaceID, taskID int64, tagIDs []int64) error
	ValidateTagsExist(ctx context.Context, db DBTX, workspaceID int64, tagIDs []int64) (bool, error)
	GetByID(ctx context.Context, db DBTX, workspaceID, id int64) (*models.Tag, error)
	Create(ctx context.Context, db DBTX, tag *models.Tag) error
	Update(ctx context.Context, db DBTX, tag *models.Tag) error
	Delete(ctx context.Context, db DBTX, workspaceID, id int64) error
	List(ctx context.Context, db DBTX, workspaceID int64, query string) ([]*models.Tag, error)
}

type tagRepository struct{}

func NewTagRepository() TagRepository {
	return &tagRepository{}
}

func (r *tagRepository) ListByTaskID(ctx context.Context, db DBTX, workspaceID, taskID int64) ([]*models.Tag, error) {
	rows, err := db.Query(ctx,
		`SELECT tg.id, tg.workspace_id, tg.name, tg.color, tg.created_at, tg.updated_at
		 FROM task_tags tt
		 JOIN tags tg ON tg.id = tt.tag_id
		 WHERE tt.workspace_id = $1 AND tt.task_id = $2
		 ORDER BY tg.name, tg.id`,
		workspaceID, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, &t)
	}
	return tags, nil
}

func (r *tagRepository) AssignToTask(ctx context.Context, db DBTX, workspaceID, taskID int64, tagIDs []int64) error {
	// Replace-all:
	_, err := db.Exec(ctx,
		`DELETE FROM task_tags WHERE workspace_id=$1 AND task_id=$2`,
		workspaceID, taskID,
	)
	if err != nil {
		return err
	}

	if len(tagIDs) > 0 {
		_, err = db.Exec(ctx,
			`INSERT INTO task_tags (workspace_id, task_id, tag_id)
			 SELECT $1, $2, unnest($3::bigint[])
			 ON CONFLICT DO NOTHING`,
			workspaceID, taskID, tagIDs,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *tagRepository) ValidateTagsExist(ctx context.Context, db DBTX, workspaceID int64, tagIDs []int64) (bool, error) {
	if len(tagIDs) == 0 {
		return true, nil
	}
	var count int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM tags WHERE workspace_id=$1 AND id = ANY($2::bigint[])`,
		workspaceID, tagIDs,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == len(tagIDs), nil
}

func (r *tagRepository) GetByID(ctx context.Context, db DBTX, workspaceID, id int64) (*models.Tag, error) {
	var t models.Tag
	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, name, color, created_at, updated_at
		 FROM tags
		 WHERE id=$1 AND workspace_id=$2`,
		id, workspaceID,
	).Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *tagRepository) Create(ctx context.Context, db DBTX, t *models.Tag) error {
	return db.QueryRow(ctx,
		`INSERT INTO tags (workspace_id, name, color)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		t.WorkspaceID, t.Name, t.Color,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *tagRepository) Update(ctx context.Context, db DBTX, t *models.Tag) error {
	_, err := db.Exec(ctx,
		`UPDATE tags SET name=$1, color=$2, updated_at=now()
		 WHERE id=$3 AND workspace_id=$4`,
		t.Name, t.Color, t.ID, t.WorkspaceID,
	)
	return err
}

func (r *tagRepository) Delete(ctx context.Context, db DBTX, workspaceID, id int64) error {
	_, err := db.Exec(ctx,
		"DELETE FROM tags WHERE id=$1 AND workspace_id=$2",
		id, workspaceID,
	)
	return err
}

func (r *tagRepository) List(ctx context.Context, db DBTX, workspaceID int64, query string) ([]*models.Tag, error) {
	sql := `SELECT id, workspace_id, name, color, created_at, updated_at
		 FROM tags
		 WHERE workspace_id=$1`
	args := []any{workspaceID}

	if query != "" {
		sql += ` AND name ILIKE '%' || $2 || '%'`
		args = append(args, query)
	}

	sql += ` ORDER BY name, id`

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, &t)
	}
	return tags, nil
}

package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type TaskRepository interface {
	GetByID(ctx context.Context, db DBTX, workspaceID, id int64) (*models.Task, error)
	Create(ctx context.Context, db DBTX, task *models.Task) error
	Update(ctx context.Context, db DBTX, task *models.Task) error
	Delete(ctx context.Context, db DBTX, workspaceID, id int64) error
	List(ctx context.Context, db DBTX, workspaceID int64, projectID *int64) ([]*models.Task, error)
	IsVisible(ctx context.Context, db DBTX, workspaceID, id int64) (bool, error)
	IsRecurringVisible(ctx context.Context, db DBTX, workspaceID, id int64) (bool, error)
}

type taskRepository struct{}

func NewTaskRepository() TaskRepository {
	return &taskRepository{}
}

func (r *taskRepository) GetByID(ctx context.Context, db DBTX, workspaceID, id int64) (*models.Task, error) {
	var t models.Task
	err := db.QueryRow(ctx,
		`SELECT t.id, t.workspace_id, t.project_id, t.title, t.description,
		        t.due_at, t.completed_at, t.deleted_at,
		        t.repeat_every, t.repeat_unit,
		        t.recurrence_start_at, t.next_due_at,
		        t.created_by, t.closed_by, t.assigned_to,
		        t.created_at, t.updated_at,
		        uc.public_id as created_by_public_id,
		        ucl.public_id as closed_by_public_id,
		        ua.public_id as assigned_to_public_id
		 FROM tasks t
		 LEFT JOIN users uc ON uc.id = t.created_by
		 LEFT JOIN users ucl ON ucl.id = t.closed_by
		 LEFT JOIN users ua ON ua.id = t.assigned_to
		 WHERE t.id=$1 AND t.workspace_id=$2 AND t.deleted_at IS NULL`,
		id, workspaceID,
	).Scan(
		&t.ID, &t.WorkspaceID, &t.ProjectID, &t.Title, &t.Description,
		&t.DueAt, &t.CompletedAt, &t.DeletedAt,
		&t.RepeatEvery, &t.RepeatUnit,
		&t.RecurrenceStartAt, &t.NextDueAt,
		&t.CreatedBy, &t.ClosedBy, &t.AssignedTo,
		&t.CreatedAt, &t.UpdatedAt,
		&t.CreatedByPublicID, &t.ClosedByPublicID, &t.AssignedToPublicID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *taskRepository) Create(ctx context.Context, db DBTX, t *models.Task) error {
	return db.QueryRow(ctx,
		`INSERT INTO tasks (
			workspace_id, project_id, title, description,
			due_at, repeat_every, repeat_unit,
			recurrence_start_at, next_due_at,
			created_by, assigned_to
		 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at, updated_at`,
		t.WorkspaceID, t.ProjectID, t.Title, t.Description,
		t.DueAt, t.RepeatEvery, t.RepeatUnit,
		t.RecurrenceStartAt, t.NextDueAt,
		t.CreatedBy, t.AssignedTo,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *taskRepository) Update(ctx context.Context, db DBTX, t *models.Task) error {
	_, err := db.Exec(ctx,
		`UPDATE tasks SET
			project_id = $1, title = $2, description = $3,
			due_at = $4, completed_at = $5,
			repeat_every = $6, repeat_unit = $7,
			recurrence_start_at = $8, next_due_at = $9,
			closed_by = $10, assigned_to = $11,
			updated_at = now()
		 WHERE id = $12 AND workspace_id = $13`,
		t.ProjectID, t.Title, t.Description,
		t.DueAt, t.CompletedAt,
		t.RepeatEvery, t.RepeatUnit,
		t.RecurrenceStartAt, t.NextDueAt,
		t.ClosedBy, t.AssignedTo,
		t.ID, t.WorkspaceID,
	)
	return err
}

func (r *taskRepository) Delete(ctx context.Context, db DBTX, workspaceID, id int64) error {
	_, err := db.Exec(ctx,
		"UPDATE tasks SET deleted_at = now(), updated_at = now() WHERE id = $1 AND workspace_id = $2",
		id, workspaceID,
	)
	return err
}

func (r *taskRepository) List(ctx context.Context, db DBTX, workspaceID int64, projectID *int64) ([]*models.Task, error) {
	query := `SELECT t.id, t.workspace_id, t.project_id, t.title, t.description,
		        t.due_at, t.completed_at, t.deleted_at,
		        t.repeat_every, t.repeat_unit,
		        t.recurrence_start_at, t.next_due_at,
		        t.created_by, t.closed_by, t.assigned_to,
		        t.created_at, t.updated_at,
		        uc.public_id as created_by_public_id,
		        ucl.public_id as closed_by_public_id,
		        ua.public_id as assigned_to_public_id
		 FROM tasks t
		 LEFT JOIN users uc ON uc.id = t.created_by
		 LEFT JOIN users ucl ON ucl.id = t.closed_by
		 LEFT JOIN users ua ON ua.id = t.assigned_to
		 WHERE t.workspace_id=$1 AND t.deleted_at IS NULL`

	args := []any{workspaceID}
	if projectID != nil {
		query += " AND t.project_id = $2"
		args = append(args, *projectID)
	}
	query += " ORDER BY t.created_at DESC"

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var t models.Task
		err := rows.Scan(
			&t.ID, &t.WorkspaceID, &t.ProjectID, &t.Title, &t.Description,
			&t.DueAt, &t.CompletedAt, &t.DeletedAt,
			&t.RepeatEvery, &t.RepeatUnit,
			&t.RecurrenceStartAt, &t.NextDueAt,
			&t.CreatedBy, &t.ClosedBy, &t.AssignedTo,
			&t.CreatedAt, &t.UpdatedAt,
			&t.CreatedByPublicID, &t.ClosedByPublicID, &t.AssignedToPublicID,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &t)
	}
	return tasks, nil
}

func (r *taskRepository) IsVisible(ctx context.Context, db DBTX, workspaceID, id int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1
		   FROM tasks t
		   JOIN projects p ON p.id = t.project_id
		   WHERE t.id = $1
		     AND t.workspace_id = $2
		     AND t.deleted_at IS NULL
		     AND p.deleted_at IS NULL
		 )`,
		id, workspaceID,
	).Scan(&ok)
	return ok, err
}

func (r *taskRepository) IsRecurringVisible(ctx context.Context, db DBTX, workspaceID, id int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1
		   FROM tasks t
		   JOIN projects p ON p.id = t.project_id
		   WHERE t.id = $1
		     AND t.workspace_id = $2
		     AND t.deleted_at IS NULL
		     AND p.deleted_at IS NULL
		     AND t.repeat_every IS NOT NULL
		     AND t.repeat_unit IS NOT NULL
		 )`,
		id, workspaceID,
	).Scan(&ok)
	return ok, err
}

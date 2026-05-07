package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type OccurrenceRepository interface {
	GetMaxDueAndIndex(ctx context.Context, db DBTX, workspaceID, taskID int64) (*time.Time, *int64, error)
	Insert(ctx context.Context, db DBTX, workspaceID, taskID int64, dueAt time.Time, index int64) (bool, int64, error)
	GetMinDueUncompleted(ctx context.Context, db DBTX, workspaceID, taskID int64, from time.Time) (*time.Time, error)
	List(ctx context.Context, db DBTX, workspaceID, taskID int64, from, to time.Time) ([]*models.Occurrence, error)
	UpdateCompletion(ctx context.Context, db DBTX, workspaceID, taskID, occID int64, completedAt *time.Time) (*models.Occurrence, error)
}

type occurrenceRepository struct{}

func NewOccurrenceRepository() OccurrenceRepository {
	return &occurrenceRepository{}
}

func (r *occurrenceRepository) GetMaxDueAndIndex(ctx context.Context, db DBTX, workspaceID, taskID int64) (*time.Time, *int64, error) {
	var lastDue *time.Time
	var lastIndex *int64
	err := db.QueryRow(ctx,
		`SELECT MAX(due_at), MAX(occurrence_index)
		 FROM task_occurrences
		 WHERE workspace_id=$1 AND task_id=$2`,
		workspaceID, taskID,
	).Scan(&lastDue, &lastIndex)
	return lastDue, lastIndex, err
}

func (r *occurrenceRepository) Insert(ctx context.Context, db DBTX, workspaceID, taskID int64, dueAt time.Time, index int64) (bool, int64, error) {
	ct, err := db.Exec(ctx,
		`INSERT INTO task_occurrences (workspace_id, task_id, due_at, occurrence_index)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (task_id, due_at) DO NOTHING`,
		workspaceID, taskID, dueAt, index,
	)
	if err != nil {
		return false, 0, err
	}

	if ct.RowsAffected() > 0 {
		return true, index, nil
	}

	// Already exists, fetch its index
	var existingIndex int64
	err = db.QueryRow(ctx,
		`SELECT occurrence_index FROM task_occurrences WHERE task_id=$1 AND due_at=$2`,
		taskID, dueAt,
	).Scan(&existingIndex)
	return false, existingIndex, err
}

func (r *occurrenceRepository) GetMinDueUncompleted(ctx context.Context, db DBTX, workspaceID, taskID int64, from time.Time) (*time.Time, error) {
	var nextDue *time.Time
	err := db.QueryRow(ctx,
		`SELECT MIN(due_at)
		 FROM task_occurrences
		 WHERE workspace_id=$1 AND task_id=$2
		   AND completed_at IS NULL
		   AND due_at >= $3`,
		workspaceID, taskID, from.UTC(),
	).Scan(&nextDue)
	return nextDue, err
}

func (r *occurrenceRepository) List(ctx context.Context, db DBTX, workspaceID, taskID int64, from, to time.Time) ([]*models.Occurrence, error) {
	rows, err := db.Query(ctx,
		`SELECT id, task_id, occurrence_index, due_at, completed_at
		 FROM task_occurrences
		 WHERE workspace_id=$1 AND task_id=$2
		   AND due_at >= $3 AND due_at <= $4
		 ORDER BY due_at, id`,
		workspaceID, taskID, from.UTC(), to.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.Occurrence
	for rows.Next() {
		var o models.Occurrence
		if err := rows.Scan(&o.ID, &o.TaskID, &o.OccurrenceIndex, &o.DueAt, &o.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, &o)
	}
	return out, nil
}

func (r *occurrenceRepository) UpdateCompletion(ctx context.Context, db DBTX, workspaceID, taskID, occID int64, completedAt *time.Time) (*models.Occurrence, error) {
	var o models.Occurrence
	err := db.QueryRow(ctx,
		`UPDATE task_occurrences
		 SET completed_at=$1, updated_at=now()
		 WHERE id=$2 AND task_id=$3 AND workspace_id=$4
		 RETURNING id, task_id, occurrence_index, due_at, completed_at`,
		completedAt, occID, taskID, workspaceID,
	).Scan(&o.ID, &o.TaskID, &o.OccurrenceIndex, &o.DueAt, &o.CompletedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

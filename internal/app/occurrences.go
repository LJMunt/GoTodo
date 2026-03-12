package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"GoToDo/internal/logging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Occurrence struct {
	ID              int64
	TaskID          int64
	OccurrenceIndex int64
	DueAt           time.Time
	CompletedAt     *time.Time
}

// EnsureOccurrencesUpTo generates missing occurrences for a recurring task up to `to` (inclusive).
// Safe to call repeatedly. Uses ON CONFLICT DO NOTHING.
func EnsureOccurrencesUpTo(ctx context.Context, db DBTX, workspaceID, taskID int64, to time.Time) error {
	l := logging.From(ctx)
	l.Debug().Int64("user_id", workspaceID).Int64("task_id", taskID).Time("to", to).Msg("ensuring task occurrences")

	var every *int
	var unit *string
	var startAt *time.Time

	err := db.QueryRow(ctx,
		`SELECT repeat_every, repeat_unit, recurrence_start_at
		 FROM tasks
		 WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		taskID, workspaceID,
	).Scan(&every, &unit, &startAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("task not found")
		}
		return err
	}

	// Not recurring -> nothing to generate
	if every == nil || unit == nil || *every <= 0 || *unit == "" {
		return nil
	}

	anchor := time.Now().UTC()
	if startAt != nil {
		anchor = startAt.UTC()
	}

	// Find the latest existing due_at and occurrence_index
	var lastDue *time.Time
	var lastIndex *int64
	if err := db.QueryRow(ctx,
		`SELECT MAX(due_at), MAX(occurrence_index)
		 FROM task_occurrences
		 WHERE workspace_id=$1 AND task_id=$2`,
		workspaceID, taskID,
	).Scan(&lastDue, &lastIndex); err != nil {
		return err
	}

	step := func(t time.Time) (time.Time, error) {
		switch *unit {
		case "day", "days":
			return t.AddDate(0, 0, *every), nil
		case "week", "weeks":
			return t.AddDate(0, 0, 7*(*every)), nil
		case "month", "months":
			return t.AddDate(0, *every, 0), nil
		default:
			return time.Time{}, fmt.Errorf("invalid repeat_unit: %s", *unit)
		}
	}

	next := anchor
	nextIndex := int64(1)
	if lastDue != nil {
		next = lastDue.UTC()
		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
	}
	if lastIndex != nil {
		nextIndex = *lastIndex + 1
	}
	to = to.UTC()
	count := 0

	// First, generate occurrences up to the requested window `to` (original behavior)
	for !next.After(to) {
		ct, err := db.Exec(ctx,
			`INSERT INTO task_occurrences (workspace_id, task_id, due_at, occurrence_index)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (task_id, due_at) DO NOTHING`,
			workspaceID, taskID, next, nextIndex,
		)
		if err != nil {
			return err
		}
		if ct.RowsAffected() > 0 {
			count++
			nextIndex++
		} else {
			// Already exists, fetch its index to keep nextIndex in sync
			if err := db.QueryRow(ctx,
				`SELECT occurrence_index FROM task_occurrences WHERE task_id=$1 AND due_at=$2`,
				taskID, next,
			).Scan(&nextIndex); err != nil {
				return err
			}
			nextIndex++
		}

		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
	}

	// If fewer than 3 occurrences were generated, extend beyond `to` to reach at least 3
	for count < 3 {
		ct, err := db.Exec(ctx,
			`INSERT INTO task_occurrences (workspace_id, task_id, due_at, occurrence_index)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (task_id, due_at) DO NOTHING`,
			workspaceID, taskID, next, nextIndex,
		)
		if err != nil {
			return err
		}
		if ct.RowsAffected() > 0 {
			count++
			nextIndex++
		} else {
			// Already exists, fetch its index to keep nextIndex in sync
			if err := db.QueryRow(ctx,
				`SELECT occurrence_index FROM task_occurrences WHERE task_id=$1 AND due_at=$2`,
				taskID, next,
			).Scan(&nextIndex); err != nil {
				return err
			}
			nextIndex++
		}
		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
		if count > 100 { // safety guard
			break
		}
	}

	if count > 0 {
		l.Debug().Int("generated", count).Msg("task occurrences generated")
	}

	now := time.Now().UTC()
	var nextDue *time.Time
	if err := db.QueryRow(ctx,
		`SELECT MIN(due_at)
		 FROM task_occurrences
		 WHERE workspace_id=$1 AND task_id=$2
		   AND completed_at IS NULL
		   AND due_at >= $3`,
		workspaceID, taskID, now,
	).Scan(&nextDue); err != nil {
		return err
	}

	_, _ = db.Exec(ctx,
		`UPDATE tasks SET next_due_at=$1, updated_at=now()
		 WHERE id=$2 AND workspace_id=$3`,
		nextDue, taskID, workspaceID,
	)

	return nil
}

func ListTaskOccurrences(ctx context.Context, db DBTX, workspaceID, taskID int64, from, to time.Time) ([]Occurrence, error) {
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

	out := make([]Occurrence, 0, 64)
	for rows.Next() {
		var o Occurrence
		if err := rows.Scan(&o.ID, &o.TaskID, &o.OccurrenceIndex, &o.DueAt, &o.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func SetOccurrenceCompleted(ctx context.Context, db DBTX, workspaceID, taskID, occID int64, completed bool) (Occurrence, error) {
	var out Occurrence

	if completed {
		now := time.Now().UTC()
		err := db.QueryRow(ctx,
			`UPDATE task_occurrences
			 SET completed_at=$1, updated_at=now()
			 WHERE id=$2 AND task_id=$3 AND workspace_id=$4
			 RETURNING id, task_id, occurrence_index, due_at, completed_at`,
			now, occID, taskID, workspaceID,
		).Scan(&out.ID, &out.TaskID, &out.OccurrenceIndex, &out.DueAt, &out.CompletedAt)
		if err != nil {
			return Occurrence{}, err
		}
		return out, nil
	}

	err := db.QueryRow(ctx,
		`UPDATE task_occurrences
		 SET completed_at=NULL, updated_at=now()
		 WHERE id=$1 AND task_id=$2 AND workspace_id=$3
		 RETURNING id, task_id, occurrence_index, due_at, completed_at`,
		occID, taskID, workspaceID,
	).Scan(&out.ID, &out.TaskID, &out.OccurrenceIndex, &out.DueAt, &out.CompletedAt)
	if err != nil {
		return Occurrence{}, err
	}
	return out, nil
}

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Occurrence struct {
	ID          int64
	TaskID      int64
	DueAt       time.Time
	CompletedAt *time.Time
}

// EnsureOccurrencesUpTo generates missing occurrences for a recurring task up to `to` (inclusive).
// Safe to call repeatedly. Uses ON CONFLICT DO NOTHING.
func EnsureOccurrencesUpTo(ctx context.Context, db DBTX, userID, taskID int64, to time.Time) error {
	var every *int
	var unit *string
	var startAt *time.Time

	err := db.QueryRow(ctx,
		`SELECT repeat_every, repeat_unit, recurrence_start_at
		 FROM tasks
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		taskID, userID,
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

	// Find the latest existing due_at
	var lastDue *time.Time
	if err := db.QueryRow(ctx,
		`SELECT MAX(due_at)
		 FROM task_occurrences
		 WHERE user_id=$1 AND task_id=$2`,
		userID, taskID,
	).Scan(&lastDue); err != nil {
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
	if lastDue != nil {
		next = lastDue.UTC()
		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
	}
	to = to.UTC()
	for !next.After(to) {
		_, err := db.Exec(ctx,
			`INSERT INTO task_occurrences (user_id, task_id, due_at)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (task_id, due_at) DO NOTHING`,
			userID, taskID, next,
		)
		if err != nil {
			return err
		}

		n, err := step(next)
		if err != nil {
			return err
		}
		next = n
	}

	now := time.Now().UTC()
	var nextDue *time.Time
	if err := db.QueryRow(ctx,
		`SELECT MIN(due_at)
		 FROM task_occurrences
		 WHERE user_id=$1 AND task_id=$2
		   AND completed_at IS NULL
		   AND due_at >= $3`,
		userID, taskID, now,
	).Scan(&nextDue); err != nil {
		return err
	}

	_, _ = db.Exec(ctx,
		`UPDATE tasks SET next_due_at=$1, updated_at=now()
		 WHERE id=$2 AND user_id=$3`,
		nextDue, taskID, userID,
	)

	return nil
}

func ListTaskOccurrences(ctx context.Context, db DBTX, userID, taskID int64, from, to time.Time) ([]Occurrence, error) {
	rows, err := db.Query(ctx,
		`SELECT id, task_id, due_at, completed_at
		 FROM task_occurrences
		 WHERE user_id=$1 AND task_id=$2
		   AND due_at >= $3 AND due_at <= $4
		 ORDER BY due_at, id`,
		userID, taskID, from.UTC(), to.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Occurrence, 0, 64)
	for rows.Next() {
		var o Occurrence
		if err := rows.Scan(&o.ID, &o.TaskID, &o.DueAt, &o.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func SetOccurrenceCompleted(ctx context.Context, db DBTX, userID, taskID, occID int64, completed bool) (Occurrence, error) {
	var out Occurrence

	if completed {
		now := time.Now().UTC()
		err := db.QueryRow(ctx,
			`UPDATE task_occurrences
			 SET completed_at=$1, updated_at=now()
			 WHERE id=$2 AND task_id=$3 AND user_id=$4
			 RETURNING id, task_id, due_at, completed_at`,
			now, occID, taskID, userID,
		).Scan(&out.ID, &out.TaskID, &out.DueAt, &out.CompletedAt)
		if err != nil {
			return Occurrence{}, err
		}
		return out, nil
	}

	err := db.QueryRow(ctx,
		`UPDATE task_occurrences
		 SET completed_at=NULL, updated_at=now()
		 WHERE id=$1 AND task_id=$2 AND user_id=$3
		 RETURNING id, task_id, due_at, completed_at`,
		occID, taskID, userID,
	).Scan(&out.ID, &out.TaskID, &out.DueAt, &out.CompletedAt)
	if err != nil {
		return Occurrence{}, err
	}
	return out, nil
}

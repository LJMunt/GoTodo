package app

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MockDB implements DBTX for testing
type MockDB struct {
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	ExecFunc     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *MockDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (m *MockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return nil, nil
}

func (m *MockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &MockRow{}
}

// MockRow implements pgx.Row
type MockRow struct {
	ScanFunc func(dest ...any) error
}

func (m *MockRow) Scan(dest ...any) error {
	if m.ScanFunc != nil {
		return m.ScanFunc(dest...)
	}
	return nil
}

func TestEnsureOccurrencesUpTo_NonRecurring(t *testing.T) {
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					// Scan into (every, unit, startAt)
					*(dest[0].(**int)) = nil
					*(dest[1].(**string)) = nil
					*(dest[2].(**time.Time)) = nil
					return nil
				},
			}
		},
	}

	err := EnsureOccurrencesUpTo(context.Background(), db, 1, 1, time.Now())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestEnsureOccurrencesUpTo_Daily(t *testing.T) {
	every := 1
	unit := "day"
	startAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	inserted := []time.Time{}

	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			// There are 3 QueryRow calls in EnsureOccurrencesUpTo
			// 1. SELECT repeat_every, ...
			// 2. SELECT MAX(due_at) ...
			// 3. SELECT MIN(due_at) ...
			if sql == `SELECT repeat_every, repeat_unit, recurrence_start_at
		 FROM tasks
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL` {
				return &MockRow{
					ScanFunc: func(dest ...any) error {
						*(dest[0].(**int)) = &every
						*(dest[1].(**string)) = &unit
						*(dest[2].(**time.Time)) = &startAt
						return nil
					},
				}
			}
			if sql == `SELECT MAX(due_at)
		 FROM task_occurrences
		 WHERE user_id=$1 AND task_id=$2` {
				return &MockRow{
					ScanFunc: func(dest ...any) error {
						*(dest[0].(**time.Time)) = nil // No previous occurrences
						return nil
					},
				}
			}
			return &MockRow{ScanFunc: func(dest ...any) error { return nil }}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if sql == `INSERT INTO task_occurrences (user_id, task_id, due_at)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (task_id, due_at) DO NOTHING` {
				inserted = append(inserted, args[2].(time.Time))
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}

	to := time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC)
	err := EnsureOccurrencesUpTo(context.Background(), db, 1, 1, to)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedCount := 3 // Jan 1, Jan 2, Jan 3
	if len(inserted) != expectedCount {
		t.Errorf("expected %d insertions, got %d", expectedCount, len(inserted))
	}

	if !inserted[0].Equal(startAt) {
		t.Errorf("expected first insertion at %v, got %v", startAt, inserted[0])
	}
	expectedLast := time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC)
	if !inserted[2].Equal(expectedLast) {
		t.Errorf("expected last insertion at %v, got %v", expectedLast, inserted[2])
	}
}

func TestEnsureOccurrencesUpTo_Monthly(t *testing.T) {
	every := 2
	unit := "months"
	startAt := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)

	inserted := []time.Time{}

	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if sql == `SELECT repeat_every, repeat_unit, recurrence_start_at
		 FROM tasks
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL` {
				return &MockRow{
					ScanFunc: func(dest ...any) error {
						*(dest[0].(**int)) = &every
						*(dest[1].(**string)) = &unit
						*(dest[2].(**time.Time)) = &startAt
						return nil
					},
				}
			}
			if sql == `SELECT MAX(due_at)
		 FROM task_occurrences
		 WHERE user_id=$1 AND task_id=$2` {
				return &MockRow{
					ScanFunc: func(dest ...any) error {
						*(dest[0].(**time.Time)) = nil
						return nil
					},
				}
			}
			return &MockRow{ScanFunc: func(dest ...any) error { return nil }}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if sql == `INSERT INTO task_occurrences (user_id, task_id, due_at)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (task_id, due_at) DO NOTHING` {
				inserted = append(inserted, args[2].(time.Time))
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}

	// Jan 15, Mar 15, May 15
	to := time.Date(2023, 5, 20, 0, 0, 0, 0, time.UTC)
	err := EnsureOccurrencesUpTo(context.Background(), db, 1, 1, to)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(inserted) != 3 {
		t.Errorf("expected 3 insertions, got %d", len(inserted))
	}
	if !inserted[1].Equal(time.Date(2023, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("expected second insertion at 2023-03-15, got %v", inserted[1])
	}
}

func TestEnsureOccurrencesUpTo_TaskNotFound(t *testing.T) {
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	err := EnsureOccurrencesUpTo(context.Background(), db, 1, 1, time.Now())
	if err == nil || err.Error() != "task not found" {
		t.Errorf("expected 'task not found' error, got %v", err)
	}
}

func TestListTaskOccurrences(t *testing.T) {
	// MockRows for Query
	// This is a bit complex since we need to mock pgx.Rows interface
	// For simplicity, let's just check if it calls Query with right arguments
	var calledFrom, calledTo time.Time
	db := &MockDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			calledFrom = args[2].(time.Time)
			calledTo = args[3].(time.Time)
			return &EmptyRows{}, nil
		},
	}

	from := time.Now()
	to := from.Add(24 * time.Hour)
	_, err := ListTaskOccurrences(context.Background(), db, 1, 1, from, to)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !calledFrom.Equal(from.UTC()) {
		t.Errorf("expected from %v, got %v", from.UTC(), calledFrom)
	}
	if !calledTo.Equal(to.UTC()) {
		t.Errorf("expected to %v, got %v", to.UTC(), calledTo)
	}
}

type EmptyRows struct {
	pgx.Rows
}

func (e *EmptyRows) Next() bool { return false }
func (e *EmptyRows) Close()     {}
func (e *EmptyRows) Err() error { return nil }

func TestSetOccurrenceCompleted(t *testing.T) {
	db := &MockDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					*(dest[0].(*int64)) = 100 // occID
					*(dest[1].(*int64)) = 1   // taskID
					return nil
				},
			}
		},
	}

	occ, err := SetOccurrenceCompleted(context.Background(), db, 1, 1, 100, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if occ.ID != 100 {
		t.Errorf("expected ID 100, got %d", occ.ID)
	}
}

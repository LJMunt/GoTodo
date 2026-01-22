package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskResponse struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"user_id"`
	ProjectID   int64   `json:"project_id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`

	// DueAt semantics:
	// - non-recurring: tasks.due_at
	// - recurring: tasks.next_due_at (next occurrence)
	DueAt       *time.Time `json:"due_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`

	RepeatEvery *int    `json:"repeat_every,omitempty"`
	RepeatUnit  *string `json:"repeat_unit,omitempty"`

	RecurrenceStartAt *time.Time `json:"recurrence_start_at,omitempty"`
	NextDueAt         *time.Time `json:"next_due_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func parseInt64Param(r *http.Request, key string) (int64, error) {
	s := chi.URLParam(r, key)
	return strconv.ParseInt(s, 10, 64)
}

func isRecurring(repeatEvery *int, repeatUnit *string) bool {
	return repeatEvery != nil && repeatUnit != nil
}

// For recurring tasks, we lazily generate occurrences up to this horizon
// to keep next_due_at accurate for UI.
func defaultHorizon() time.Time {
	return time.Now().UTC().AddDate(0, 0, 60)
}

func CreateTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Title       string     `json:"title"`
		Description *string    `json:"description"`
		DueAt       *time.Time `json:"due_at"`
		RepeatEvery *int       `json:"repeat_every"`
		RepeatUnit  *string    `json:"repeat_unit"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Title == "" {
			writeErr(w, http.StatusBadRequest, "title is required")
			return
		}

		// recurrence validation
		if (req.RepeatEvery == nil) != (req.RepeatUnit == nil) {
			writeErr(w, http.StatusBadRequest, "repeat_every and repeat_unit must be set together")
			return
		}
		if req.RepeatEvery != nil && *req.RepeatEvery <= 0 {
			writeErr(w, http.StatusBadRequest, "repeat_every must be > 0")
			return
		}

		recurring := isRecurring(req.RepeatEvery, req.RepeatUnit)
		if recurring && req.DueAt == nil {
			// For recurring tasks, the first due date anchors the series.
			writeErr(w, http.StatusBadRequest, "due_at is required for recurring tasks")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()

		// Ensure project exists, belongs to user, and is not deleted
		var projectOK bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM projects
			   WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
			 )`,
			projectID, user.ID,
		).Scan(&projectOK); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to verify project")
			return
		}
		if !projectOK {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Insert task (template or normal)
		var t TaskResponse

		var dueAtForTasks *time.Time
		var recurrenceStartAt *time.Time
		var nextDueAt *time.Time

		if recurring {
			ra := req.DueAt.UTC()
			recurrenceStartAt = &ra
			nextDueAt = &ra
			dueAtForTasks = nil // templates do not use tasks.due_at for recurrence
		} else {
			if req.DueAt != nil {
				d := req.DueAt.UTC()
				dueAtForTasks = &d
			}
		}

		err = tx.QueryRow(ctx,
			`INSERT INTO tasks (user_id, project_id, title, description, due_at, repeat_every, repeat_unit, recurrence_start_at, next_due_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			 RETURNING id, user_id, project_id, title, description,
			           due_at, completed_at, deleted_at,
			           repeat_every, repeat_unit,
			           recurrence_start_at, next_due_at,
			           created_at, updated_at`,
			user.ID, projectID, req.Title, req.Description,
			dueAtForTasks, req.RepeatEvery, req.RepeatUnit,
			recurrenceStartAt, nextDueAt,
		).Scan(
			&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description,
			&t.DueAt, &t.CompletedAt, &t.DeletedAt,
			&t.RepeatEvery, &t.RepeatUnit,
			&t.RecurrenceStartAt, &t.NextDueAt,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to create task")
			return
		}

		// If recurring, ensure the first occurrence exists (history basis)
		if recurring {
			_, err := tx.Exec(ctx,
				`INSERT INTO task_occurrences (user_id, task_id, due_at)
				 VALUES ($1,$2,$3)
				 ON CONFLICT (task_id, due_at) DO NOTHING`,
				user.ID, t.ID, t.RecurrenceStartAt.UTC(),
			)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to create initial occurrence")
				return
			}

			// Generate ahead a bit and update next_due_at cache
			if err := app.EnsureOccurrencesUpTo(ctx, tx, user.ID, t.ID, defaultHorizon()); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to initialize occurrences")
				return
			}

			// Refresh next_due_at for response
			_ = tx.QueryRow(ctx,
				`SELECT next_due_at FROM tasks WHERE id=$1 AND user_id=$2`,
				t.ID, user.ID,
			).Scan(&t.NextDueAt)

			// For response, DueAt should represent "next due"
			t.DueAt = t.NextDueAt
			t.CompletedAt = nil
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to commit task")
			return
		}

		writeJSON(w, http.StatusCreated, t)
	}
}

func ListProjectTasksHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Project must exist and not be deleted
		var projectOK bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM projects
			   WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
			 )`,
			projectID, user.ID,
		).Scan(&projectOK); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to verify project")
			return
		}
		if !projectOK {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}

		rows, err := db.Query(ctx,
			`SELECT id, user_id, project_id, title, description,
			        due_at, completed_at, deleted_at,
			        repeat_every, repeat_unit,
			        recurrence_start_at, next_due_at,
			        created_at, updated_at
			 FROM tasks
			 WHERE user_id=$1 AND project_id=$2 AND deleted_at IS NULL
			 ORDER BY id`,
			user.ID, projectID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list tasks")
			return
		}
		defer rows.Close()

		out := make([]TaskResponse, 0, 64)
		recurringTaskIDs := make([]int64, 0, 16)

		for rows.Next() {
			var t TaskResponse
			if err := rows.Scan(
				&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description,
				&t.DueAt, &t.CompletedAt, &t.DeletedAt,
				&t.RepeatEvery, &t.RepeatUnit,
				&t.RecurrenceStartAt, &t.NextDueAt,
				&t.CreatedAt, &t.UpdatedAt,
			); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read tasks")
				return
			}

			if isRecurring(t.RepeatEvery, t.RepeatUnit) {
				recurringTaskIDs = append(recurringTaskIDs, t.ID)
				t.DueAt = t.NextDueAt
				t.CompletedAt = nil
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read tasks")
			return
		}

		// Lazy generation to keep next_due_at up to date.
		h := defaultHorizon()
		for _, taskID := range recurringTaskIDs {
			_ = app.EnsureOccurrencesUpTo(ctx, db, user.ID, taskID, h) // best-effort
		}

		if len(recurringTaskIDs) > 0 {
			rows2, err := db.Query(ctx,
				`SELECT id, next_due_at
				 FROM tasks
				 WHERE user_id=$1 AND id = ANY($2)`,
				user.ID, recurringTaskIDs,
			)
			if err == nil {
				defer rows2.Close()
				nextMap := map[int64]*time.Time{}
				for rows2.Next() {
					var id int64
					var nd *time.Time
					_ = rows2.Scan(&id, &nd)
					if nd != nil {
						t := nd.UTC()
						nextMap[id] = &t
					} else {
						nextMap[id] = nil
					}
				}
				for i := range out {
					if isRecurring(out[i].RepeatEvery, out[i].RepeatUnit) {
						out[i].NextDueAt = nextMap[out[i].ID]
						out[i].DueAt = out[i].NextDueAt
					}
				}
			}
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func GetTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()

		var t TaskResponse
		err = db.QueryRow(ctx,
			`SELECT t.id, t.user_id, t.project_id, t.title, t.description,
			        t.due_at, t.completed_at, t.deleted_at,
			        t.repeat_every, t.repeat_unit,
			        t.recurrence_start_at, t.next_due_at,
			        t.created_at, t.updated_at
			 FROM tasks t
			 JOIN projects p ON p.id = t.project_id
			 WHERE t.id=$1 AND t.user_id=$2 AND t.deleted_at IS NULL AND p.deleted_at IS NULL`,
			taskID, user.ID,
		).Scan(
			&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description,
			&t.DueAt, &t.CompletedAt, &t.DeletedAt,
			&t.RepeatEvery, &t.RepeatUnit,
			&t.RecurrenceStartAt, &t.NextDueAt,
			&t.CreatedAt, &t.UpdatedAt,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch task")
			return
		}

		if isRecurring(t.RepeatEvery, t.RepeatUnit) {
			_ = app.EnsureOccurrencesUpTo(ctx, db, user.ID, t.ID, defaultHorizon()) // best-effort

			_ = db.QueryRow(ctx,
				`SELECT next_due_at FROM tasks WHERE id=$1 AND user_id=$2`,
				t.ID, user.ID,
			).Scan(&t.NextDueAt)

			t.DueAt = t.NextDueAt
			t.CompletedAt = nil
		}

		writeJSON(w, http.StatusOK, t)
	}
}

func UpdateTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`

		DueAt      *time.Time `json:"due_at"` // set
		ClearDueAt *bool      `json:"clear_due_at"`

		Completed *bool `json:"completed"`

		RepeatEvery *int    `json:"repeat_every"`
		RepeatUnit  *string `json:"repeat_unit"`
		ClearRepeat *bool   `json:"clear_repeat"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Title != nil && *req.Title == "" {
			writeErr(w, http.StatusBadRequest, "title cannot be empty")
			return
		}

		// Recurrence rules:
		// - if clear_repeat=true => set both to NULL
		// - else: repeat_every and repeat_unit must be set together (or both nil = no change)
		clearRepeat := req.ClearRepeat != nil && *req.ClearRepeat
		if clearRepeat {
			req.RepeatEvery = nil
			req.RepeatUnit = nil
		} else if (req.RepeatEvery == nil) != (req.RepeatUnit == nil) {
			writeErr(w, http.StatusBadRequest, "repeat_every and repeat_unit must be set together")
			return
		} else if req.RepeatEvery != nil && *req.RepeatEvery <= 0 {
			writeErr(w, http.StatusBadRequest, "repeat_every must be > 0")
			return
		}

		clearDue := req.ClearDueAt != nil && *req.ClearDueAt

		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var (
			projectID int64

			curTitle       string
			curDesc        *string
			curDueAt       *time.Time
			curCompletedAt *time.Time

			curRepeatEvery *int
			curRepeatUnit  *string

			curRecStart *time.Time
			curNextDue  *time.Time
		)

		err = tx.QueryRow(ctx,
			`SELECT t.project_id, t.title, t.description, t.due_at, t.completed_at,
			        t.repeat_every, t.repeat_unit,
			        t.recurrence_start_at, t.next_due_at
			 FROM tasks t
			 JOIN projects p ON p.id = t.project_id
			 WHERE t.id=$1 AND t.user_id=$2 AND t.deleted_at IS NULL AND p.deleted_at IS NULL`,
			taskID, user.ID,
		).Scan(
			&projectID, &curTitle, &curDesc, &curDueAt, &curCompletedAt,
			&curRepeatEvery, &curRepeatUnit,
			&curRecStart, &curNextDue,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch task")
			return
		}

		wasRecurring := isRecurring(curRepeatEvery, curRepeatUnit)

		// Compute what recurrence will be after update
		newRepeatEvery := curRepeatEvery
		newRepeatUnit := curRepeatUnit

		if clearRepeat {
			newRepeatEvery = nil
			newRepeatUnit = nil
		} else if req.RepeatEvery != nil && req.RepeatUnit != nil {
			// set recurrence
			newRepeatEvery = req.RepeatEvery
			newRepeatUnit = req.RepeatUnit
		}
		willBeRecurring := isRecurring(newRepeatEvery, newRepeatUnit)

		// Completion on recurring templates is not allowed.
		if req.Completed != nil && (wasRecurring || willBeRecurring) {
			writeErr(w, http.StatusBadRequest, "cannot complete a recurring task template; complete an occurrence instead")
			return
		}

		// Apply title/description updates
		newTitle := curTitle
		if req.Title != nil {
			newTitle = *req.Title
		}
		newDesc := curDesc
		if req.Description != nil {
			newDesc = req.Description
		}

		// Apply due changes depending on type
		var newDueAt *time.Time = curDueAt
		if clearDue {
			newDueAt = nil
		} else if req.DueAt != nil {
			d := req.DueAt.UTC()
			newDueAt = &d
		}

		// Completion timestamp for non-recurring tasks only
		var newCompletedAt *time.Time = curCompletedAt
		if req.Completed != nil {
			if *req.Completed {
				now := time.Now().UTC()
				newCompletedAt = &now
			} else {
				newCompletedAt = nil
			}
		}

		var newRecStart *time.Time = curRecStart
		var newNextDue *time.Time = curNextDue
		var storedTasksDueAt *time.Time // what goes into tasks.due_at column

		switch {
		case !wasRecurring && !willBeRecurring:
			// Normal -> Normal
			storedTasksDueAt = newDueAt

		case wasRecurring && willBeRecurring:
			// Recurring -> Recurring
			if req.DueAt != nil {
				d := req.DueAt.UTC()
				newRecStart = &d
				_, err := tx.Exec(ctx,
					`DELETE FROM task_occurrences
					 WHERE user_id=$1 AND task_id=$2 AND completed_at IS NULL AND due_at >= $3`,
					user.ID, taskID, d,
				)
				if err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to reset future occurrences")
					return
				}
			}
			// Templates do not store tasks.due_at
			storedTasksDueAt = nil
			newCompletedAt = nil

		case !wasRecurring && willBeRecurring:
			// Normal -> Recurring
			anchor := newDueAt
			if anchor == nil {
				writeErr(w, http.StatusBadRequest, "due_at is required to enable recurrence")
				return
			}
			a := anchor.UTC()
			newRecStart = &a
			storedTasksDueAt = nil
			newCompletedAt = nil

			// Ensure anchor occurrence exists
			_, err := tx.Exec(ctx,
				`INSERT INTO task_occurrences (user_id, task_id, due_at)
				 VALUES ($1,$2,$3)
				 ON CONFLICT (task_id, due_at) DO NOTHING`,
				user.ID, taskID, a,
			)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to create initial occurrence")
				return
			}

		case wasRecurring && !willBeRecurring:
			// Recurring -> Normal
			// Pick a concrete due_at:
			// - if user set due_at explicitly, use it
			// - else prefer next_due_at
			// - else recurrence_start_at
			// - else keep existing tasks.due_at (unlikely for recurring)
			var chosen *time.Time
			if req.DueAt != nil {
				d := req.DueAt.UTC()
				chosen = &d
			} else if curNextDue != nil {
				d := curNextDue.UTC()
				chosen = &d
			} else if curRecStart != nil {
				d := curRecStart.UTC()
				chosen = &d
			} else {
				chosen = newDueAt
			}
			storedTasksDueAt = chosen
			newRecStart = nil
			newNextDue = nil
			// keep completion changes for normal tasks
		}

		// Persist task changes atomically
		err = tx.QueryRow(ctx,
			`UPDATE tasks
			 SET title=$1,
			     description=$2,
			     due_at=$3,
			     completed_at=$4,
			     repeat_every=$5,
			     repeat_unit=$6,
			     recurrence_start_at=$7,
			     next_due_at=$8,
			     updated_at=now()
			 WHERE id=$9 AND user_id=$10 AND deleted_at IS NULL
			 RETURNING next_due_at`,
			newTitle,
			newDesc,
			storedTasksDueAt,
			newCompletedAt,
			newRepeatEvery,
			newRepeatUnit,
			newRecStart,
			newNextDue,
			taskID,
			user.ID,
		).Scan(&newNextDue)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update task")
			return
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to commit task update")
			return
		}

		// If it's recurring after update, generate occurrences and refresh next_due_at cache.
		if willBeRecurring {
			_ = app.EnsureOccurrencesUpTo(ctx, db, user.ID, taskID, defaultHorizon())
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`UPDATE tasks SET deleted_at=now(), updated_at=now()
			 WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`,
			taskID, user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete task")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

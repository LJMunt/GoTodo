package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskResponse struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	ProjectID   int64      `json:"project_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`

	RepeatEvery *int    `json:"repeat_every,omitempty"`
	RepeatUnit  *string `json:"repeat_unit,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func parseInt64Param(r *http.Request, key string) (int64, error) {
	s := chi.URLParam(r, key)
	return strconv.ParseInt(s, 10, 64)
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
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			http.Error(w, "invalid project id", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, "title is required", http.StatusBadRequest)
			return
		}

		// recurrence validation
		if (req.RepeatEvery == nil) != (req.RepeatUnit == nil) {
			http.Error(w, "repeat_every and repeat_unit must be set together", http.StatusBadRequest)
			return
		}
		if req.RepeatEvery != nil && *req.RepeatEvery <= 0 {
			http.Error(w, "repeat_every must be > 0", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// ✅ Ensure project exists, belongs to user, and is not deleted
		var projectOK bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM projects
			   WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
			 )`,
			projectID, user.ID,
		).Scan(&projectOK); err != nil {
			http.Error(w, "failed to verify project", http.StatusInternalServerError)
			return
		}
		if !projectOK {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		var t TaskResponse
		err = db.QueryRow(ctx,
			`INSERT INTO tasks (user_id, project_id, title, description, due_at, repeat_every, repeat_unit)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, user_id, project_id, title, description, due_at, completed_at, deleted_at,
			           repeat_every, repeat_unit, created_at, updated_at`,
			user.ID, projectID, req.Title, req.Description, req.DueAt, req.RepeatEvery, req.RepeatUnit,
		).Scan(
			&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description, &t.DueAt, &t.CompletedAt, &t.DeletedAt,
			&t.RepeatEvery, &t.RepeatUnit, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			http.Error(w, "failed to create task", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(t)
	}
}

func ListProjectTasksHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			http.Error(w, "invalid project id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// ✅ Project must exist and not be deleted
		var projectOK bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM projects
			   WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
			 )`,
			projectID, user.ID,
		).Scan(&projectOK); err != nil {
			http.Error(w, "failed to verify project", http.StatusInternalServerError)
			return
		}
		if !projectOK {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		rows, err := db.Query(ctx,
			`SELECT id, user_id, project_id, title, description, due_at, completed_at, deleted_at,
			        repeat_every, repeat_unit, created_at, updated_at
			 FROM tasks
			 WHERE user_id=$1 AND project_id=$2 AND deleted_at IS NULL
			 ORDER BY id`,
			user.ID, projectID,
		)
		if err != nil {
			http.Error(w, "failed to list tasks", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		tasks := make([]TaskResponse, 0, 64)
		for rows.Next() {
			var t TaskResponse
			if err := rows.Scan(
				&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description, &t.DueAt, &t.CompletedAt, &t.DeletedAt,
				&t.RepeatEvery, &t.RepeatUnit, &t.CreatedAt, &t.UpdatedAt,
			); err != nil {
				http.Error(w, "failed to read tasks", http.StatusInternalServerError)
				return
			}
			tasks = append(tasks, t)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "failed to read tasks", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tasks)
	}
}

func GetTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		taskID, err := parseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			http.Error(w, "invalid task id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var t TaskResponse
		err = db.QueryRow(ctx,
			`SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.due_at, t.completed_at, t.deleted_at,
			        t.repeat_every, t.repeat_unit, t.created_at, t.updated_at
			 FROM tasks t
			 JOIN projects p ON p.id = t.project_id
			 WHERE t.id=$1 AND t.user_id=$2 AND t.deleted_at IS NULL AND p.deleted_at IS NULL`,
			taskID, user.ID,
		).Scan(
			&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description, &t.DueAt, &t.CompletedAt, &t.DeletedAt,
			&t.RepeatEvery, &t.RepeatUnit, &t.CreatedAt, &t.UpdatedAt,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to fetch task", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(t)
	}
}

func UpdateTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Title       *string    `json:"title"`
		Description *string    `json:"description"`
		DueAt       *time.Time `json:"due_at"` // set
		ClearDueAt  *bool      `json:"clear_due_at"`

		Completed *bool `json:"completed"`

		RepeatEvery *int    `json:"repeat_every"`
		RepeatUnit  *string `json:"repeat_unit"`
		ClearRepeat *bool   `json:"clear_repeat"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		taskID, err := parseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			http.Error(w, "invalid task id", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Title != nil && *req.Title == "" {
			http.Error(w, "title cannot be empty", http.StatusBadRequest)
			return
		}

		// Recurrence rules:
		// - if clear_repeat=true => set both to NULL
		// - else: repeat_every and repeat_unit must be set together (or both nil = no change)
		if req.ClearRepeat != nil && *req.ClearRepeat {
			req.RepeatEvery = nil
			req.RepeatUnit = nil
		} else if (req.RepeatEvery == nil) != (req.RepeatUnit == nil) {
			http.Error(w, "repeat_every and repeat_unit must be set together", http.StatusBadRequest)
			return
		} else if req.RepeatEvery != nil && *req.RepeatEvery <= 0 {
			http.Error(w, "repeat_every must be > 0", http.StatusBadRequest)
			return
		}

		// due_at clearing
		var dueAt pgtype.Timestamptz
		dueAt.Valid = false // means "no change" unless we set otherwise

		// We'll use a little trick: pass three-valued parameters and decide in SQL.
		// In pgx, easiest is: use separate params for set/clear.
		clearDue := req.ClearDueAt != nil && *req.ClearDueAt

		// completion timestamp logic
		// If completed is nil => no change
		// If completed=true => completed_at=now()
		// If completed=false => completed_at=NULL
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// We need current completion state only if you want to avoid bumping updated_at when nothing changes.
		// For MVP, we keep it simple.

		// Handle repeat params: if clear_repeat=true, we’ll explicitly null them in SQL.
		// We'll pass two booleans: clear_repeat, clear_due
		// and two optional values: due_at, repeat_every/unit.
		var tag pgconn.CommandTag
		_ = tag

		// Use Exec with SQL branching.
		_, err = db.Exec(ctx,
			`UPDATE tasks
			 SET title = COALESCE($1, title),
			     description = COALESCE($2, description),
			     due_at = CASE
			               WHEN $3::boolean THEN NULL
			               WHEN $4::timestamptz IS NOT NULL THEN $4
			               ELSE due_at
			             END,
			     completed_at = CASE
			                    WHEN $5::boolean IS NULL THEN completed_at
			                    WHEN $5::boolean = true THEN now()
			                    ELSE completed_at
			                  END,
			     repeat_every = CASE
			                     WHEN $6::boolean THEN NULL
			                     WHEN $7::int IS NOT NULL THEN $7
			                     ELSE repeat_every
			                   END,
			     repeat_unit = CASE
			                    WHEN $6::boolean THEN NULL
			                    WHEN $8::text IS NOT NULL THEN $8
			                    ELSE repeat_unit
			                  END,
			     updated_at = now()
			 WHERE id=$9 AND user_id=$10 AND deleted_at IS NULL`,
			req.Title,
			req.Description,
			clearDue,
			req.DueAt,
			req.Completed,
			req.ClearRepeat != nil && *req.ClearRepeat,
			req.RepeatEvery,
			req.RepeatUnit,
			taskID,
			user.ID,
		)
		if err != nil {
			http.Error(w, "failed to update task", http.StatusInternalServerError)
			return
		}

		// We want 404 if no row matched
		var exists bool
		err = db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`,
			taskID, user.ID,
		).Scan(&exists)
		if err == nil && !exists {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		taskID, err := parseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			http.Error(w, "invalid task id", http.StatusBadRequest)
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
			http.Error(w, "failed to delete task", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

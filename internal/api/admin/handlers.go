package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type ProjectResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type UserResponse struct {
	ID        int64      `json:"id"`
	Email     string     `json:"email"`
	IsAdmin   bool       `json:"is_admin"`
	IsActive  bool       `json:"is_active"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type TaskResponse struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	ProjectID   int64      `json:"project_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	RepeatEvery *int       `json:"repeat_every,omitempty"`
	RepeatUnit  *string    `json:"repeat_unit,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TagResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DatabaseMetricsResponse struct {
	DatabaseSize  string  `json:"database_size"`
	Connections   int     `json:"connections"`
	Deadlocks     int     `json:"deadlocks"`
	BlocksRead    int64   `json:"blocks_read"`
	BlocksHit     int64   `json:"blocks_hit"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, apiError{Error: msg})
}

func parseInt64Param(r *http.Request, key string) (int64, error) {
	s := chi.URLParam(r, key)
	return strconv.ParseInt(s, 10, 64)
}

func ListUsersHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		activeStr := strings.TrimSpace(r.URL.Query().Get("active"))
		limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))
		offsetStr := strings.TrimSpace(r.URL.Query().Get("offset"))

		// defaults
		limit := 50
		if limitStr != "" {
			l, err := strconv.Atoi(limitStr)
			if err != nil || l <= 0 {
				writeErr(w, "invalid limit", http.StatusBadRequest)
				return
			}
			if l > 200 {
				l = 200
			}
			limit = l
		}

		offset := 0
		if offsetStr != "" {
			o, err := strconv.Atoi(offsetStr)
			if err != nil || o < 0 {
				writeErr(w, "invalid offset", http.StatusBadRequest)
				return
			}
			offset = o
		}

		var activeFilter *bool
		if activeStr != "" {
			a, err := strconv.ParseBool(activeStr)
			if err != nil {
				writeErr(w, "invalid active (use true/false)", http.StatusBadRequest)
				return
			}
			activeFilter = &a
		}

		baseQuery := `SELECT id, email, is_admin, is_active, last_login, created_at, updated_at FROM users`
		where := make([]string, 0, 2)
		args := make([]any, 0, 4)

		// helper to add a clause with correct $N numbering
		addClause := func(clause string, arg any) {
			where = append(where, fmt.Sprintf(clause, len(args)+1))
			args = append(args, arg)
		}

		if q != "" {
			addClause("email ILIKE $%d", "%"+q+"%")
		}

		if activeFilter != nil {
			addClause("is_active = $%d", *activeFilter)
		}

		query := baseQuery
		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}

		// LIMIT/OFFSET always present
		query += fmt.Sprintf(" ORDER BY id ASC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, query, args...)
		if err != nil {
			writeErr(w, "failed to list users", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := make([]UserResponse, 0, limit)
		for rows.Next() {
			var u UserResponse
			if err := rows.Scan(&u.ID, &u.Email, &u.IsAdmin, &u.IsActive, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
				writeErr(w, "failed to scan user", http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, "failed to read users", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(users)
	}
}

func GetUserHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var u UserResponse
		err := db.QueryRow(ctx,
			`SELECT id, email, is_admin, is_active, last_login, created_at, updated_at FROM users WHERE id = $1`,
			idStr,
		).Scan(&u.ID, &u.Email, &u.IsAdmin, &u.IsActive, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeErr(w, "failed to fetch user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u)
	}
}

func UpdateUserHandler(db *pgxpool.Pool) http.HandlerFunc {
	type updateRequest struct {
		IsAdmin  *bool   `json:"is_admin"`
		IsActive *bool   `json:"is_active"`
		Password *string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var req updateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, "invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if req.IsAdmin != nil {
			_, err := db.Exec(ctx, "UPDATE users SET is_admin = $1, updated_at = now() WHERE id = $2", *req.IsAdmin, idStr)
			if err != nil {
				writeErr(w, "failed to update is_admin", http.StatusInternalServerError)
				return
			}
		}

		if req.IsActive != nil {
			_, err := db.Exec(ctx, "UPDATE users SET is_active = $1, updated_at = now() WHERE id = $2", *req.IsActive, idStr)
			if err != nil {
				writeErr(w, "failed to update is_active", http.StatusInternalServerError)
				return
			}
		}

		if req.Password != nil {
			if len(*req.Password) < 8 {
				writeErr(w, "password too short", http.StatusBadRequest)
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
			if err != nil {
				writeErr(w, "failed to hash password", http.StatusInternalServerError)
				return
			}
			_, err = db.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2", string(hash), idStr)
			if err != nil {
				writeErr(w, "failed to update password", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteUserHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx, "UPDATE users SET is_active=false, updated_at=now() WHERE id=$1;", idStr)
		if err != nil {
			writeErr(w, "failed to delete user", http.StatusInternalServerError)
			return
		}

		if tag.RowsAffected() == 0 {
			writeErr(w, "user not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func ListUserProjectsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}

		includeDeleted := r.URL.Query().Get("include_deleted") == "true"

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `SELECT id, name, description, created_at, updated_at
			 FROM projects
			 WHERE user_id=$1`
		if !includeDeleted {
			query += " AND deleted_at IS NULL"
		}
		query += " ORDER BY id"

		rows, err := db.Query(ctx, query, userID)
		if err != nil {
			writeErr(w, "failed to list projects", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		projects := make([]ProjectResponse, 0, 32)
		for rows.Next() {
			var p ProjectResponse
			if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
				writeErr(w, "failed to scan project", http.StatusInternalServerError)
				return
			}
			projects = append(projects, p)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, "failed to read projects", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projects)
	}
}

func GetProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, "invalid project id", http.StatusBadRequest)
			return
		}

		includeDeleted := r.URL.Query().Get("include_deleted") == "true"

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var p ProjectResponse
		query := `SELECT id, name, description, created_at, updated_at
			 FROM projects
			 WHERE id = $1 AND user_id = $2`
		if !includeDeleted {
			query += " AND deleted_at IS NULL"
		}

		err = db.QueryRow(ctx, query, projectID, userID).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, "project not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeErr(w, "failed to fetch project", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)
	}
}

func UpdateProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, "invalid project id", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name != nil && *req.Name == "" {
			writeErr(w, "name cannot be empty", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`UPDATE projects
			 SET name = COALESCE($1, name),
			     description = COALESCE($2, description),
			     updated_at = now()
			 WHERE id = $3 AND user_id = $4 AND deleted_at IS NULL`,
			req.Name, req.Description, projectID, userID,
		)
		if err != nil {
			writeErr(w, "failed to update project", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, "project not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, "invalid project id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		tag, err := tx.Exec(ctx,
			`UPDATE projects
			 SET deleted_at = now(), updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			projectID, userID,
		)
		if err != nil {
			writeErr(w, "failed to delete project", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, "project not found", http.StatusNotFound)
			return
		}

		_, err = tx.Exec(ctx,
			`UPDATE tasks
			 SET deleted_at = now(), updated_at = now()
			 WHERE project_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			projectID, userID,
		)
		if err != nil {
			writeErr(w, "failed to delete project tasks", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, "failed to commit project delete", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func RestoreProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		RestoreTasks *bool `json:"restore_tasks"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, "invalid project id", http.StatusBadRequest)
			return
		}

		// Default: restore tasks too (fits your deletion semantics)
		restoreTasks := true
		var req request
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if req.RestoreTasks != nil {
				restoreTasks = *req.RestoreTasks
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		tag, err := tx.Exec(ctx,
			`UPDATE projects
			 SET deleted_at = NULL, updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND deleted_at IS NOT NULL`,
			projectID, userID,
		)
		if err != nil {
			writeErr(w, "failed to restore project", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, "project not found or not deleted", http.StatusNotFound)
			return
		}

		if restoreTasks {
			_, err := tx.Exec(ctx,
				`UPDATE tasks
				 SET deleted_at = NULL, updated_at = now()
				 WHERE project_id = $1 AND user_id = $2 AND deleted_at IS NOT NULL`,
				projectID, userID,
			)
			if err != nil {
				writeErr(w, "failed to restore project tasks", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, "failed to commit restore", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func RestoreUserTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		taskID, err := parseInt64Param(r, "taskId")
		if err != nil || taskID <= 0 {
			writeErr(w, "invalid task id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check the task exists (even if deleted) and whether its project is deleted
		var projectDeleted bool
		err = db.QueryRow(ctx,
			`SELECT (p.deleted_at IS NOT NULL) AS project_deleted
			 FROM tasks t
			 JOIN projects p ON p.id = t.project_id
			 WHERE t.id = $1 AND t.user_id = $2`,
			taskID, userID,
		).Scan(&projectDeleted)

		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, "task not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeErr(w, "failed to verify task", http.StatusInternalServerError)
			return
		}

		if projectDeleted {
			writeErr(w, "cannot restore task while its project is deleted (restore project first)", http.StatusConflict)
			return
		}

		tag, err := db.Exec(ctx,
			`UPDATE tasks
			 SET deleted_at = NULL, updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND deleted_at IS NOT NULL`,
			taskID, userID,
		)
		if err != nil {
			writeErr(w, "failed to restore task", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, "task not found or not deleted", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func ListUserTasksHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}

		includeDeleted := r.URL.Query().Get("include_deleted") == "true"
		includeDeletedProjects := includeDeleted

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Join projects so we can optionally filter on project deleted_at too.
		query := `
			SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.due_at, t.completed_at, t.deleted_at,
			       t.repeat_every, t.repeat_unit, t.created_at, t.updated_at
			FROM tasks t
			JOIN projects p ON p.id = t.project_id
			WHERE t.user_id = $1
		`

		if !includeDeleted {
			query += " AND t.deleted_at IS NULL"
		}
		if !includeDeletedProjects {
			query += " AND p.deleted_at IS NULL"
		}
		query += " ORDER BY t.id"

		rows, err := db.Query(ctx, query, userID)
		if err != nil {
			writeErr(w, "failed to list tasks", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := make([]TaskResponse, 0, 64)
		for rows.Next() {
			var t TaskResponse
			if err := rows.Scan(
				&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description,
				&t.DueAt, &t.CompletedAt, &t.DeletedAt,
				&t.RepeatEvery, &t.RepeatUnit,
				&t.CreatedAt, &t.UpdatedAt,
			); err != nil {
				writeErr(w, "failed to read tasks", http.StatusInternalServerError)
				return
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, "failed to read tasks", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func ListProjectTasksHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		projectID, err := parseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			writeErr(w, "invalid project id", http.StatusBadRequest)
			return
		}

		includeDeleted := r.URL.Query().Get("include_deleted") == "true"

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// ✅ Verify project exists, belongs to user, and is not deleted (unless include_deleted=true)
		var projectOK bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM projects
			   WHERE id=$1 AND user_id=$2 AND ($3::boolean = true OR deleted_at IS NULL)
			 )`,
			projectID, userID, includeDeleted,
		).Scan(&projectOK); err != nil {
			writeErr(w, "failed to verify project", http.StatusInternalServerError)
			return
		}
		if !projectOK {
			writeErr(w, "project not found", http.StatusNotFound)
			return
		}

		query := `
			SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.due_at, t.completed_at, t.deleted_at,
			       t.repeat_every, t.repeat_unit, t.created_at, t.updated_at
			FROM tasks t
			JOIN projects p ON p.id = t.project_id
			WHERE t.user_id = $1 AND t.project_id = $2
		`
		if !includeDeleted {
			query += " AND t.deleted_at IS NULL"
		}
		// If project is deleted and includeDeleted=true, we allow listing.
		// If includeDeleted=false, projectOK already ensured it's not deleted.
		query += " ORDER BY t.id"

		rows, err := db.Query(ctx, query, userID, projectID)
		if err != nil {
			writeErr(w, "failed to list tasks", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := make([]TaskResponse, 0, 64)
		for rows.Next() {
			var t TaskResponse
			if err := rows.Scan(
				&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description,
				&t.DueAt, &t.CompletedAt, &t.DeletedAt,
				&t.RepeatEvery, &t.RepeatUnit,
				&t.CreatedAt, &t.UpdatedAt,
			); err != nil {
				writeErr(w, "failed to read tasks", http.StatusInternalServerError)
				return
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, "failed to read tasks", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func DeleteUserTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		taskID, err := parseInt64Param(r, "taskId")
		if err != nil || taskID <= 0 {
			writeErr(w, "invalid task id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`UPDATE tasks
			 SET deleted_at = now(), updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			taskID, userID,
		)
		if err != nil {
			writeErr(w, "failed to delete task", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, "task not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func ListUserTagsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `
			SELECT id, name, created_at, updated_at
			FROM tags
			WHERE user_id = $1`
		args := []any{userID}
		if q != "" {
			query += ` AND name ILIKE '%' || $2 || '%'`
			args = append(args, q)
		}
		query += ` ORDER BY name, id`

		rows, err := db.Query(ctx, query, args...)
		if err != nil {
			writeErr(w, "failed to list tags", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := make([]TagResponse, 0, 64)
		for rows.Next() {
			var t TagResponse
			if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
				writeErr(w, "failed to read tags", http.StatusInternalServerError)
				return
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, "failed to read tags", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func DeleteUserTagHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseInt64Param(r, "userId")
		if err != nil || userID <= 0 {
			writeErr(w, "invalid user id", http.StatusBadRequest)
			return
		}
		tagID, err := parseInt64Param(r, "tagId")
		if err != nil || tagID <= 0 {
			writeErr(w, "invalid tag id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `DELETE FROM tags WHERE id = $1 AND user_id = $2`
		tag, err := db.Exec(ctx, query, tagID, userID)
		if err != nil {
			writeErr(w, "failed to delete tag", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, "tag not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type adminDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func GetDatabaseMetricsHandler(db adminDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var sizeBytes int64
		err := db.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&sizeBytes)
		if err != nil {
			writeErr(w, "failed to get database size", http.StatusInternalServerError)
			return
		}

		var numbackends int
		var deadlocks int64
		var blksRead int64
		var blksHit int64
		err = db.QueryRow(ctx, `
			SELECT
			  numbackends,
			  deadlocks,
			  blks_read,
			  blks_hit
			FROM pg_stat_database
			WHERE datname = current_database();
		`).Scan(&numbackends, &deadlocks, &blksRead, &blksHit)
		if err != nil {
			writeErr(w, "failed to get database stats", http.StatusInternalServerError)
			return
		}

		var cacheHitRatio float64
		if blksRead+blksHit > 0 {
			cacheHitRatio = float64(blksHit) / float64(blksRead+blksHit) * 100
		}

		resp := DatabaseMetricsResponse{
			DatabaseSize:  formatBytes(sizeBytes),
			Connections:   numbackends,
			Deadlocks:     int(deadlocks),
			BlocksRead:    blksRead,
			BlocksHit:     blksHit,
			CacheHitRatio: cacheHitRatio,
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	if b < unit*unit {
		return fmt.Sprintf("%.2f KB", float64(b)/float64(unit))
	}
	if b < unit*unit*unit {
		return fmt.Sprintf("%.2f MB", float64(b)/float64(unit*unit))
	}
	if b < unit*unit*unit*unit {
		return fmt.Sprintf("%.2f GB", float64(b)/float64(unit*unit*unit))
	}
	return fmt.Sprintf("%.2f TB", float64(b)/float64(unit*unit*unit*unit))
}

package projects

import (
	authmw "GoToDo/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type apiError struct {
	Error string `json:"error"`
}

type projectQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type projectUpdater interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func CreateProjectHandler(db projectQuerier) http.HandlerFunc {
	type request struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var p ProjectResponse
		err := db.QueryRow(ctx,
			`INSERT INTO projects (user_id, name, description)
			 VALUES ($1, $2, $3)
			 RETURNING id, name, description, created_at, updated_at`,
			user.ID, req.Name, req.Description,
		).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)

		if err != nil {
			// Specific error for unique constraint violation
			if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "23505") {
				writeErr(w, http.StatusConflict, "project with this name already exists")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to create project")
			return
		}

		writeJSON(w, http.StatusCreated, p)
	}
}

func ListProjectsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, `SELECT id, name, description, created_at, updated_at FROM projects WHERE user_id=$1 AND deleted_at IS NULL ORDER BY id`, user.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list projects")
			return
		}
		defer rows.Close()

		projects := make([]ProjectResponse, 0, 16)
		for rows.Next() {
			var p ProjectResponse
			if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to scan project")
				return
			}
			projects = append(projects, p)
		}
		writeJSON(w, http.StatusOK, projects)
	}

}

func GetProjectHandler(db projectQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := parseID(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var p ProjectResponse
		err = db.QueryRow(ctx,
			`SELECT id, name, description, created_at, updated_at
			 FROM projects
			 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			id, user.ID,
		).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch project")
			return
		}

		writeJSON(w, http.StatusOK, p)
	}
}

func UpdateProjectHandler(db projectUpdater) http.HandlerFunc {
	type request struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := parseID(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`UPDATE projects
			 SET name = COALESCE($1, name),
			     description = COALESCE($2, description),
			     updated_at = now()
			 WHERE id = $3 AND user_id = $4`,
			req.Name, req.Description, id, user.ID,
		)

		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update project")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := parseID(r) // your existing helper
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// 1) soft delete project (only if not already deleted)
		tag, err := tx.Exec(ctx,
			`UPDATE projects
			 SET deleted_at = now(), updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			id, user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete project")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}

		// 2) soft delete tasks under project (only tasks not already deleted)
		_, err = tx.Exec(ctx,
			`UPDATE tasks
			 SET deleted_at = now(), updated_at = now()
			 WHERE project_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			id, user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete project tasks")
			return
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to commit project delete")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

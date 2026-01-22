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
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func CreateProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
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
				http.Error(w, "project with this name already exists", http.StatusConflict)
				return
			}
			http.Error(w, "failed to create project", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(p)
	}
}

func ListProjectsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, `SELECT id, name, description, created_at, updated_at FROM projects WHERE user_id=$1 AND deleted_at IS NULL ORDER BY id`, user.ID)
		if err != nil {
			http.Error(w, "failed to list projects", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var projects []ProjectResponse
		for rows.Next() {
			var p ProjectResponse
			if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
				http.Error(w, "failed to scan project", http.StatusInternalServerError)
				return
			}
			projects = append(projects, p)
		}
		_ = json.NewEncoder(w).Encode(projects)
	}

}

func GetProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid project id", http.StatusBadRequest)
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
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to fetch project", http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(p)
	}
}

func UpdateProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id, err := parseID(r)
		if err != nil {
			http.Error(w, "invalid project id", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
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
			http.Error(w, "failed to update project", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteProjectHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id, err := parseID(r) // your existing helper
		if err != nil || id <= 0 {
			http.Error(w, "invalid project id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
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
			http.Error(w, "failed to delete project", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "project not found", http.StatusNotFound)
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
			http.Error(w, "failed to delete project tasks", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(ctx); err != nil {
			http.Error(w, "failed to commit project delete", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

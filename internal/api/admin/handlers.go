package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type UserResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
				http.Error(w, "invalid limit", http.StatusBadRequest)
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
				http.Error(w, "invalid offset", http.StatusBadRequest)
				return
			}
			offset = o
		}

		var activeFilter *bool
		if activeStr != "" {
			a, err := strconv.ParseBool(activeStr)
			if err != nil {
				http.Error(w, "invalid active (use true/false)", http.StatusBadRequest)
				return
			}
			activeFilter = &a
		}

		baseQuery := `SELECT id, email, is_admin, is_active, created_at, updated_at FROM users`
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
			http.Error(w, "failed to list users", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := make([]UserResponse, 0, limit)
		for rows.Next() {
			var u UserResponse
			if err := rows.Scan(&u.ID, &u.Email, &u.IsAdmin, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
				http.Error(w, "failed to scan user", http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "failed to read users", http.StatusInternalServerError)
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
			`SELECT id, email, is_admin, is_active, created_at, updated_at FROM users WHERE id = $1`,
			idStr,
		).Scan(&u.ID, &u.Email, &u.IsAdmin, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)

		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
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
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if req.IsAdmin != nil {
			_, err := db.Exec(ctx, "UPDATE users SET is_admin = $1, updated_at = now() WHERE id = $2", *req.IsAdmin, idStr)
			if err != nil {
				http.Error(w, "failed to update is_admin", http.StatusInternalServerError)
				return
			}
		}

		if req.IsActive != nil {
			_, err := db.Exec(ctx, "UPDATE users SET is_active = $1, updated_at = now() WHERE id = $2", *req.IsActive, idStr)
			if err != nil {
				http.Error(w, "failed to update is_active", http.StatusInternalServerError)
				return
			}
		}

		if req.Password != nil {
			if len(*req.Password) < 8 {
				http.Error(w, "password too short", http.StatusBadRequest)
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "failed to hash password", http.StatusInternalServerError)
				return
			}
			_, err = db.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2", string(hash), idStr)
			if err != nil {
				http.Error(w, "failed to update password", http.StatusInternalServerError)
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
			http.Error(w, "failed to delete user", http.StatusInternalServerError)
			return
		}

		if tag.RowsAffected() == 0 {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

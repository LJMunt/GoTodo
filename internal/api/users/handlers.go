package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type userSettings struct {
	Theme                string `json:"theme"`
	ShowCompletedDefault bool   `json:"showCompletedDefault"`
}

type userMeResponse struct {
	ID        int64        `json:"id"`
	Email     string       `json:"email"`
	IsAdmin   bool         `json:"is_admin"`
	IsActive  bool         `json:"is_active"`
	LastLogin *time.Time   `json:"last_login"`
	Settings  userSettings `json:"settings"`
}

type apiError struct {
	Error string `json:"error"`
}

type userDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
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

func MeHandler(db userDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		var res userMeResponse
		err := db.QueryRow(ctx,
			`SELECT email, is_admin, is_active, last_login, ui_theme, show_completed_default 
			 FROM users WHERE id=$1`,
			u.ID,
		).Scan(&res.Email, &res.IsAdmin, &res.IsActive, &res.LastLogin, &res.Settings.Theme, &res.Settings.ShowCompletedDefault)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusNotFound, "user not found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to fetch user")
			return
		}
		res.ID = u.ID

		writeJSON(w, http.StatusOK, res)
	}
}

func UpdateMeHandler(db userDB) http.HandlerFunc {
	type updateRequest struct {
		Email    *string       `json:"email"`
		Settings *userSettings `json:"settings"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		var req updateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if req.Email != nil {
			email := strings.TrimSpace(strings.ToLower(*req.Email))
			if email == "" {
				writeErr(w, http.StatusBadRequest, "email cannot be empty")
				return
			}
			_, err := db.Exec(ctx, "UPDATE users SET email = $1, updated_at = now() WHERE id = $2", email, u.ID)
			if err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					writeErr(w, http.StatusConflict, "email already exists")
					return
				}
				writeErr(w, http.StatusInternalServerError, "failed to update email")
				return
			}
		}

		if req.Settings != nil {
			if req.Settings.Theme != "" {
				if req.Settings.Theme != "system" && req.Settings.Theme != "light" && req.Settings.Theme != "dark" {
					writeErr(w, http.StatusBadRequest, "invalid theme")
					return
				}
				_, err := db.Exec(ctx, "UPDATE users SET ui_theme = $1, updated_at = now() WHERE id = $2", req.Settings.Theme, u.ID)
				if err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to update theme")
					return
				}
			}
			// showCompletedDefault is always present if Settings is provided because it's a bool,
			// but the OpenAPI says anyOf [email, settings].
			// We'll update it if settings is provided.
			_, err := db.Exec(ctx, "UPDATE users SET show_completed_default = $1, updated_at = now() WHERE id = $2", req.Settings.ShowCompletedDefault, u.ID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to update showCompletedDefault")
				return
			}
		}

		// Fetch and return updated user
		var res userMeResponse
		err := db.QueryRow(ctx,
			`SELECT email, is_admin, is_active, last_login, ui_theme, show_completed_default 
			 FROM users WHERE id=$1`,
			u.ID,
		).Scan(&res.Email, &res.IsAdmin, &res.IsActive, &res.LastLogin, &res.Settings.Theme, &res.Settings.ShowCompletedDefault)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch updated user")
			return
		}
		res.ID = u.ID
		writeJSON(w, http.StatusOK, res)
	}
}

func DeleteMeHandler(db userDB) http.HandlerFunc {
	type deleteRequest struct {
		CurrentPassword string `json:"currentPassword"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		var req deleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var passwordHash string
		var isAdmin bool
		err := db.QueryRow(ctx, "SELECT password_hash, is_admin FROM users WHERE id = $1", u.ID).Scan(&passwordHash, &isAdmin)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch user")
			return
		}

		if isAdmin {
			writeErr(w, http.StatusForbidden, "admin accounts cannot delete themselves")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		_, err = db.Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete account")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

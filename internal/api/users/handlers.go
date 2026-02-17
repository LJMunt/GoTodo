package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	authmw "GoToDo/internal/auth"
	"GoToDo/internal/logging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type userSettings struct {
	Theme                string `json:"theme"`
	ShowCompletedDefault bool   `json:"showCompletedDefault"`
	Language             string `json:"language"`
}

type userMeResponse struct {
	PublicID        string       `json:"public_id"`
	Email           string       `json:"email"`
	IsAdmin         bool         `json:"is_admin"`
	IsActive        bool         `json:"is_active"`
	LastLogin       *time.Time   `json:"last_login"`
	EmailVerifiedAt *time.Time   `json:"email_verified_at"`
	Settings        userSettings `json:"settings"`
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
			`SELECT public_id, email, is_admin, is_active, last_login, email_verified_at, ui_theme, show_completed_default, language 
			 FROM users WHERE id=$1`,
			u.ID,
		).Scan(&res.PublicID, &res.Email, &res.IsAdmin, &res.IsActive, &res.LastLogin, &res.EmailVerifiedAt, &res.Settings.Theme, &res.Settings.ShowCompletedDefault, &res.Settings.Language)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusNotFound, "user not found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to fetch user")
			return
		}
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

		l := logging.From(ctx)
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
					l.Debug().Str("email", email).Msg("user update failed: email already exists")
					writeErr(w, http.StatusConflict, "email already exists")
					return
				}
				l.Error().Err(err).Int64("user_id", u.ID).Msg("user update failed: database error")
				writeErr(w, http.StatusInternalServerError, "failed to update email")
				return
			}
			l.Info().Int64("user_id", u.ID).Str("new_email", email).Msg("user email updated")
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

			if req.Settings.Language != "" {
				_, err := db.Exec(ctx, "UPDATE users SET language = $1, updated_at = now() WHERE id = $2", req.Settings.Language, u.ID)
				if err != nil {
					var pgErr *pgconn.PgError
					if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
						writeErr(w, http.StatusBadRequest, "invalid language code")
						return
					}
					writeErr(w, http.StatusInternalServerError, "failed to update language")
					return
				}
			}
		}

		// Fetch and return updated user
		var updatedRes userMeResponse
		err := db.QueryRow(ctx,
			`SELECT public_id, email, is_admin, is_active, last_login, email_verified_at, ui_theme, show_completed_default, language 
			 FROM users WHERE id=$1`,
			u.ID,
		).Scan(&updatedRes.PublicID, &updatedRes.Email, &updatedRes.IsAdmin, &updatedRes.IsActive, &updatedRes.LastLogin, &updatedRes.EmailVerifiedAt, &updatedRes.Settings.Theme, &updatedRes.Settings.ShowCompletedDefault, &updatedRes.Settings.Language)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch updated user")
			return
		}
		writeJSON(w, http.StatusOK, updatedRes)
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

		l := logging.From(ctx)
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
			l.Debug().Int64("user_id", u.ID).Msg("account deletion failed: incorrect password")
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		_, err = db.Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID)
		if err != nil {
			l.Error().Err(err).Int64("user_id", u.ID).Msg("account deletion failed: database error")
			writeErr(w, http.StatusInternalServerError, "failed to delete account")
			return
		}

		l.Info().Int64("user_id", u.ID).Msg("user account deleted")
		w.WriteHeader(http.StatusNoContent)
	}
}

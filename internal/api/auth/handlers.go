package auth

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

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

type userCreatedResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Token string `json:"token"`
}

type apiError struct {
	Error string `json:"error"`
}

type authDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

const minPasswordLen = 8

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func SignupHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" {
			writeErr(w, http.StatusBadRequest, "email is required")
			return
		}
		if len(req.Password) < minPasswordLen {
			writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var id int64
		err = db.QueryRow(ctx,
			`INSERT INTO users (email, password_hash) VALUES ($1, $2)
			 RETURNING id`,
			email, string(hashedPassword),
		).Scan(&id)

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
				writeErr(w, http.StatusConflict, "email already exists")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		token, err := authmw.SignToken(id) // ✅ only userID in JWT now
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		writeJSON(w, http.StatusCreated, userCreatedResponse{
			ID:    id,
			Email: email,
			Token: token,
		})
	}
}

func LoginHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" || req.Password == "" {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var (
			id           int64
			passwordHash string
			isActive     bool
		)

		err := db.QueryRow(ctx,
			`SELECT id, password_hash, is_active
			 FROM users
			 WHERE email=$1`,
			email,
		).Scan(&id, &passwordHash, &isActive)

		// Don’t leak whether email exists.
		if err != nil || !isActive {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		// Update last_login
		_, _ = db.Exec(ctx, "UPDATE users SET last_login = now() WHERE id = $1", id)

		token, err := authmw.SignToken(id) // ✅ only userID in JWT now
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token})
	}
}

func PasswordChangeHandler(db authDB) http.HandlerFunc {
	type passwordChangeRequest struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		var req passwordChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if len(req.NewPassword) < minPasswordLen {
			writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var passwordHash string
		err := db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id = $1", u.ID).Scan(&passwordHash)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch user")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to hash new password")
			return
		}

		_, err = db.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2", string(newHash), u.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update password")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

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

		token, err := authmw.SignToken(id) // ✅ only userID in JWT now
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token})
	}
}

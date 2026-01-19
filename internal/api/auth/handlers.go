package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

const minPasswordLen = 8

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func SignupHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}
		if len(req.Password) < minPasswordLen {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
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
				http.Error(w, "email already exists", http.StatusConflict)
				return
			}
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}

		token, err := authmw.SignToken(id) // ✅ only userID in JWT now
		if err != nil {
			http.Error(w, "failed to sign token", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, userCreatedResponse{
			ID:    id,
			Email: email,
			Token: token,
		})
	}
}

func LoginHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" || req.Password == "" {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
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
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := authmw.SignToken(id) // ✅ only userID in JWT now
		if err != nil {
			http.Error(w, "failed to sign token", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token})
	}
}

package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
)

type apiError struct {
	Error string `json:"error"`
}

type userDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
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

		var email string
		var isAdmin bool
		var isActive bool
		err := db.QueryRow(ctx,
			`SELECT email, is_admin, is_active FROM users WHERE id=$1`,
			u.ID,
		).Scan(&email, &isAdmin, &isActive)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusNotFound, "user not found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to fetch user")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":        u.ID,
			"email":     email,
			"is_admin":  isAdmin,
			"is_active": isActive,
		})
	}
}

package users

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MeHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
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
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        u.ID,
			"email":     email,
			"is_admin":  isAdmin,
			"is_active": isActive,
		})
	}
}

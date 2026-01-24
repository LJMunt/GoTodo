package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

var userKey ctxKey

type User struct {
	ID      int64
	IsAdmin bool
}

type apiError struct {
	Error string `json:"error"`
}

func FromContext(ctx context.Context) (User, bool) {
	v := ctx.Value(userKey)
	u, ok := v.(User)
	return u, ok
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func bearerToken(r *http.Request) (string, bool) {
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(strings.TrimPrefix(authz, prefix))
	return tok, tok != ""
}

func RequireAuth(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearerToken(r)
			if !ok {
				writeErr(w, http.StatusUnauthorized, "missing token")
				return
			}

			claims, err := ParseToken(tok)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			var isAdmin bool
			var isActive bool
			err = db.QueryRow(ctx,
				`SELECT is_admin, is_active FROM users WHERE id=$1`,
				claims.UserID,
			).Scan(&isAdmin, &isActive)

			if err != nil || !isActive {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}

			user := User{ID: claims.UserID, IsAdmin: isAdmin}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := FromContext(r.Context())
		if !ok || !u.IsAdmin {
			writeErr(w, http.StatusForbidden, "forbidden: admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

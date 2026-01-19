package auth

import (
	"context"
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

func FromContext(ctx context.Context) (User, bool) {
	v := ctx.Value(userKey)
	u, ok := v.(User)
	return u, ok
}

func bearerToken(r *http.Request) (string, bool) {
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return "", false
	}
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
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			claims, err := ParseToken(tok)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
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

			// Don’t leak whether the account exists vs disabled.
			if err != nil || !isActive {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user := User{ID: claims.UserID, IsAdmin: isAdmin}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
		})
	}
}

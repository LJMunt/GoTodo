package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ctxKey struct{}

var userKey ctxKey

type User struct {
	ID             int64
	PublicID       string
	IsAdmin        bool
	TokenID        string
	TokenExpiresAt time.Time
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

type dbExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func RequireAuth(db dbExecutor) func(http.Handler) http.Handler {
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
			if claims.ExpiresAt == nil {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			revoked, err := isTokenRevoked(ctx, db, claims.ID)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if revoked {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}

			var isAdmin bool
			var isActive bool
			var publicID string
			err = db.QueryRow(ctx,
				`SELECT is_admin, is_active, public_id FROM users WHERE id=$1`,
				claims.UserID,
			).Scan(&isAdmin, &isActive, &publicID)

			if err != nil || !isActive {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}

			user := User{
				ID:             claims.UserID,
				IsAdmin:        isAdmin,
				PublicID:       publicID,
				TokenID:        claims.ID,
				TokenExpiresAt: claims.ExpiresAt.Time,
			}
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

func ReadOnly(db dbExecutor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method := r.Method
			path := r.URL.Path

			// Always allow GET, HEAD, OPTIONS
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Always allow login
			if method == http.MethodPost && strings.HasSuffix(path, "/auth/login") {
				next.ServeHTTP(w, r)
				return
			}

			// Always allow admin endpoints
			if strings.HasPrefix(path, "/api/v1/admin") {
				next.ServeHTTP(w, r)
				return
			}

			u, ok := FromContext(r.Context())
			if ok && u.IsAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// Check instance.readOnly in DB
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			var readOnly bool
			err := db.QueryRow(ctx, `SELECT value_json FROM config_keys WHERE key = 'instance.readOnly'`).Scan(&readOnly)
			if err != nil {
				// If we can't find the key, assume false (safe default)
				next.ServeHTTP(w, r)
				return
			}

			if readOnly {
				writeErr(w, http.StatusForbidden, "instance is in read-only mode")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isTokenRevoked(ctx context.Context, db dbExecutor, jti string) (bool, error) {
	var revoked bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM jwt_revocations
			WHERE jti = $1 AND expires_at > NOW()
		)
	`, jti).Scan(&revoked)
	if err != nil {
		return false, err
	}
	return revoked, nil
}

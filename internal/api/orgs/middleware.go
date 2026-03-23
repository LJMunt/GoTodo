package orgs

import (
	"context"
	"net/http"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
)

type dbExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func RequireOrgAdmin(db dbExecutor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authmw.FromContext(r.Context())
			if !ok {
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			orgID, err := parseInt64Param(r, "id")
			if err != nil || orgID <= 0 {
				writeErr(w, http.StatusBadRequest, "invalid organization id")
				return
			}

			var role string
			err = db.QueryRow(r.Context(),
				`SELECT om.role FROM org_members om JOIN orgs o ON o.id = om.org_id WHERE om.org_id = $1 AND om.user_id = $2 AND o.deleted_at IS NULL`,
				orgID, user.ID,
			).Scan(&role)
			if err != nil {
				writeErr(w, http.StatusForbidden, "forbidden: not a member of this organization")
				return
			}

			if role != "admin" {
				writeErr(w, http.StatusForbidden, "forbidden: organization admin access required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireOrgMember(db dbExecutor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authmw.FromContext(r.Context())
			if !ok {
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			orgID, err := parseInt64Param(r, "id")
			if err != nil || orgID <= 0 {
				writeErr(w, http.StatusBadRequest, "invalid organization id")
				return
			}

			var exists bool
			err = db.QueryRow(r.Context(),
				`SELECT EXISTS(
				   SELECT 1
				   FROM org_members om
				   JOIN orgs o ON o.id = om.org_id
				   WHERE om.org_id = $1 AND om.user_id = $2 AND o.deleted_at IS NULL
				)`,
				orgID, user.ID,
			).Scan(&exists)
			if err != nil || !exists {
				writeErr(w, http.StatusForbidden, "forbidden: not a member of this organization")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func OrganizationsEnabled(db dbExecutor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var enabled bool
			err := db.QueryRow(r.Context(),
				`SELECT value_json FROM config_keys WHERE key = 'features.organizations'`,
			).Scan(&enabled)
			if err != nil || !enabled {
				writeErr(w, http.StatusNotFound, "not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
)

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

type dbExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ResolveWorkspace resolves and authorizes the active workspace from the X-Workspace-ID header.
func ResolveWorkspace(db dbExecutor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authmw.FromContext(r.Context())
			if !ok {
				// If not authenticated, we can't resolve workspace.
				// This middleware should be used after RequireAuth.
				next.ServeHTTP(w, r)
				return
			}

			workspacePublicID := strings.TrimSpace(r.Header.Get("X-Workspace-ID"))
			// If no header or same as current (personal), do nothing.
			if workspacePublicID == "" || workspacePublicID == user.WorkspacePublicID {
				next.ServeHTTP(w, r)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			// Resolve the requested workspace
			var requestedID int64
			var workspaceType string
			var ownerUserID *int64
			var ownerOrgID *int64

			err := db.QueryRow(ctx,
				`SELECT id, type, user_id, org_id FROM workspaces WHERE public_id = $1`,
				workspacePublicID,
			).Scan(&requestedID, &workspaceType, &ownerUserID, &ownerOrgID)

			if err != nil {
				if err == pgx.ErrNoRows {
					writeErr(w, http.StatusNotFound, "workspace not found")
					return
				}
				writeErr(w, http.StatusInternalServerError, "failed to resolve workspace")
				return
			}

			// Authorization logic
			if workspaceType == "user" {
				// Personal workspace can only be accessed by its owner
				if ownerUserID == nil || *ownerUserID != user.ID {
					writeErr(w, http.StatusForbidden, "forbidden: unauthorized workspace access")
					return
				}
			} else if workspaceType == "org" {
				// Check organizations feature flag
				var enabled bool
				err := db.QueryRow(ctx,
					`SELECT value_json FROM config_keys WHERE key = 'features.organizations'`,
				).Scan(&enabled)
				if err != nil || !enabled {
					writeErr(w, http.StatusNotFound, "organizations feature is disabled")
					return
				}

				if ownerOrgID == nil {
					writeErr(w, http.StatusInternalServerError, "invalid organization workspace")
					return
				}

				// Check organization membership and whether the organization is soft-deleted
				var isMember bool
				err = db.QueryRow(ctx,
					`SELECT EXISTS (
						SELECT 1 FROM org_members om
						JOIN orgs o ON o.id = om.org_id
						WHERE om.org_id = $1 AND om.user_id = $2 AND o.deleted_at IS NULL
					)`,
					*ownerOrgID, user.ID,
				).Scan(&isMember)

				if err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to verify organization access")
					return
				}

				if !isMember {
					writeErr(w, http.StatusForbidden, "forbidden: unauthorized workspace access")
					return
				}
			} else {
				writeErr(w, http.StatusInternalServerError, "invalid workspace type")
				return
			}

			// Update user in context with the requested workspace info
			user.WorkspaceID = requestedID
			user.WorkspacePublicID = workspacePublicID
			next.ServeHTTP(w, r.WithContext(authmw.WithUser(r.Context(), user)))
		})
	}
}

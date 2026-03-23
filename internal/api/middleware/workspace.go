package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResolveWorkspace resolves and authorizes the active workspace from the X-Workspace-ID header.
func ResolveWorkspace(db *pgxpool.Pool) func(http.Handler) http.Handler {
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
					http.Error(w, "workspace not found", http.StatusNotFound)
					return
				}
				http.Error(w, "failed to resolve workspace", http.StatusInternalServerError)
				return
			}

			// Authorization logic
			if workspaceType == "user" {
				// Personal workspace can only be accessed by its owner
				if ownerUserID == nil || *ownerUserID != user.ID {
					http.Error(w, "forbidden: unauthorized workspace access", http.StatusForbidden)
					return
				}
			} else if workspaceType == "org" {
				if ownerOrgID == nil {
					http.Error(w, "invalid organization workspace", http.StatusInternalServerError)
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
					http.Error(w, "failed to verify organization access", http.StatusInternalServerError)
					return
				}

				if !isMember {
					http.Error(w, "forbidden: unauthorized workspace access", http.StatusForbidden)
					return
				}
			} else {
				http.Error(w, "invalid workspace type", http.StatusInternalServerError)
				return
			}

			// Update user in context with the requested workspace info
			user.WorkspaceID = requestedID
			user.WorkspacePublicID = workspacePublicID
			next.ServeHTTP(w, r.WithContext(authmw.WithUser(r.Context(), user)))
		})
	}
}

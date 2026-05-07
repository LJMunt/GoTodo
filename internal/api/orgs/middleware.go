package orgs

import (
	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
	"net/http"
)

func RequireOrgAdmin(deps app.Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authmw.FromContext(r.Context())
			if !ok {
				apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			orgID, err := apiutil.ParseInt64Param(r, "id")
			if err != nil || orgID <= 0 {
				apiutil.WriteErr(w, http.StatusBadRequest, "invalid organization id")
				return
			}

			role, err := deps.OrgService.GetMemberRole(r.Context(), user.ID, orgID)
			if err != nil {
				apiutil.HandleServiceErr(w, err)
				return
			}

			if role != models.RoleAdmin {
				apiutil.WriteErr(w, http.StatusForbidden, "forbidden: organization admin access required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireOrgMember(deps app.Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authmw.FromContext(r.Context())
			if !ok {
				apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			orgID, err := apiutil.ParseInt64Param(r, "id")
			if err != nil || orgID <= 0 {
				apiutil.WriteErr(w, http.StatusBadRequest, "invalid organization id")
				return
			}

			role, err := deps.OrgService.GetMemberRole(r.Context(), user.ID, orgID)
			if err != nil {
				apiutil.HandleServiceErr(w, err)
				return
			}

			if role == "" {
				apiutil.WriteErr(w, http.StatusForbidden, "forbidden: not a member of this organization")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func OrganizationsEnabled(deps app.Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var enabled bool
			err := deps.DB.QueryRow(r.Context(),
				`SELECT value_json FROM config_keys WHERE key = 'features.organizations'`,
			).Scan(&enabled)
			if err != nil || !enabled {
				apiutil.WriteErr(w, http.StatusNotFound, "not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package orgs

import (
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
)

type OrganizationResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MemberResponse struct {
	UserPublicID string    `json:"user_id"`
	UserEmail    string    `json:"email"`
	Role         string    `json:"role"`
	JoinedAt     time.Time `json:"joined_at"`
}

func mapOrgToResponse(o *models.Organization) OrganizationResponse {
	return OrganizationResponse{
		ID:        o.ID,
		Name:      o.Name,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

func CreateOrganizationHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		org, err := deps.OrgService.CreateOrganization(r.Context(), user.ID, req.Name)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusCreated, mapOrgToResponse(org))
	}
}

func ListOrganizationsHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		orgs, err := deps.OrgService.ListOrganizations(r.Context(), user.ID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := make([]OrganizationResponse, 0, len(orgs))
		for _, o := range orgs {
			resp = append(resp, mapOrgToResponse(o))
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

func GetOrganizationHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		org, err := deps.OrgService.GetOrganization(r.Context(), user.ID, orgID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapOrgToResponse(org))
	}
}

func UpdateOrganizationHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
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

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		org, err := deps.OrgService.UpdateOrganization(r.Context(), user.ID, orgID, req.Name)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapOrgToResponse(org))
	}
}

func DeleteOrganizationHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if err := deps.OrgService.DeleteOrganization(r.Context(), user.ID, orgID); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func AddMemberHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		UserPublicID string `json:"user_id"`
		Role         string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
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

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		role := models.OrgRole(req.Role)
		if role != models.RoleAdmin && role != models.RoleMember {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid role")
			return
		}

		if err := deps.OrgService.AddMember(r.Context(), user.ID, orgID, req.UserPublicID, role); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func RemoveMemberHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		UserPublicID string `json:"user_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
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

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := deps.OrgService.RemoveMember(r.Context(), user.ID, orgID, req.UserPublicID); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func LeaveOrganizationHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if err := deps.OrgService.LeaveOrganization(r.Context(), user.ID, orgID); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func ListMembersHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		members, err := deps.OrgService.ListMembers(r.Context(), user.ID, orgID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := make([]MemberResponse, 0, len(members))
		for _, m := range members {
			resp = append(resp, MemberResponse{
				UserPublicID: m.UserPublicID,
				UserEmail:    m.UserEmail,
				Role:         string(m.Role),
				JoinedAt:     m.JoinedAt,
			})
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

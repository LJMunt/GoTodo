package projects

import (
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
)

type ProjectResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func mapProjectToResponse(p *models.Project) ProjectResponse {
	return ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func CreateProjectHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
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

		p, err := deps.ProjectService.CreateProject(r.Context(), user.WorkspaceID, req.Name, req.Description)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusCreated, mapProjectToResponse(p))
	}
}

func ListProjectsHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		includeDeleted := r.URL.Query().Get("include_deleted") == "true"
		projects, err := deps.ProjectService.ListProjects(r.Context(), user.WorkspaceID, includeDeleted)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := make([]ProjectResponse, 0, len(projects))
		for _, p := range projects {
			resp = append(resp, mapProjectToResponse(p))
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

func GetProjectHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := apiutil.ParseInt64Param(r, "id")
		if err != nil || id <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		p, err := deps.ProjectService.GetProject(r.Context(), user.WorkspaceID, id)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapProjectToResponse(p))
	}
}

func UpdateProjectHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := apiutil.ParseInt64Param(r, "id")
		if err != nil || id <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		p, err := deps.ProjectService.UpdateProject(r.Context(), user.WorkspaceID, id, req.Name, req.Description)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapProjectToResponse(p))
	}
}

func DeleteProjectHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := apiutil.ParseInt64Param(r, "id")
		if err != nil || id <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		if err := deps.ProjectService.DeleteProject(r.Context(), user.WorkspaceID, id); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

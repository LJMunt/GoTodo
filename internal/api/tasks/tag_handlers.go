package tasks

import (
	"encoding/json"
	"net/http"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
)

type TaskTagResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func mapTagToResponse(t *models.Tag) TaskTagResponse {
	return TaskTagResponse{
		ID:    t.ID,
		Name:  t.Name,
		Color: t.Color,
	}
}

func GetTaskTagsHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := apiutil.ParseInt64Param(r, "taskId")
		if err != nil || taskID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		tags, err := deps.TaskService.GetTaskTags(r.Context(), user.WorkspaceID, taskID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		out := make([]TaskTagResponse, 0, len(tags))
		for _, t := range tags {
			out = append(out, mapTagToResponse(t))
		}

		apiutil.WriteJSON(w, http.StatusOK, out)
	}
}

func PutTaskTagsHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		TagIDs []int64 `json:"tag_ids"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := apiutil.ParseInt64Param(r, "taskId")
		if err != nil || taskID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.TagIDs == nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "tag_ids is required")
			return
		}

		// De-dupe + validate positive IDs
		seen := make(map[int64]struct{}, len(req.TagIDs))
		tagIDs := make([]int64, 0, len(req.TagIDs))
		for _, id := range req.TagIDs {
			if id <= 0 {
				apiutil.WriteErr(w, http.StatusBadRequest, "tag_ids must be positive integers")
				return
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			tagIDs = append(tagIDs, id)
		}

		tags, err := deps.TaskService.UpdateTaskTags(r.Context(), user.WorkspaceID, taskID, tagIDs)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		out := make([]TaskTagResponse, 0, len(tags))
		for _, t := range tags {
			out = append(out, mapTagToResponse(t))
		}

		apiutil.WriteJSON(w, http.StatusOK, out)
	}
}

package tasks

import (
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
	"GoToDo/internal/service"
)

type TaskResponse struct {
	ID                int64      `json:"id"`
	WorkspacePublicID string     `json:"workspace_id"`
	ProjectID         int64      `json:"project_id"`
	Title             string     `json:"title"`
	Description       *string    `json:"description,omitempty"`
	DueAt             *time.Time `json:"due_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
	RepeatEvery       *int       `json:"repeat_every,omitempty"`
	RepeatUnit        *string    `json:"repeat_unit,omitempty"`
	RecurrenceStartAt *time.Time `json:"recurrence_start_at,omitempty"`
	NextDueAt         *time.Time `json:"next_due_at,omitempty"`
	CreatedBy         string     `json:"created_by"`
	ClosedBy          *string    `json:"closed_by,omitempty"`
	AssignedTo        *string    `json:"assigned_to,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func mapTaskToResponse(t *models.Task, workspacePublicID string) TaskResponse {
	return TaskResponse{
		ID:                t.ID,
		WorkspacePublicID: workspacePublicID,
		ProjectID:         t.ProjectID,
		Title:             t.Title,
		Description:       t.Description,
		DueAt:             t.DueAt,
		CompletedAt:       t.CompletedAt,
		DeletedAt:         t.DeletedAt,
		RepeatEvery:       t.RepeatEvery,
		RepeatUnit:        t.RepeatUnit,
		RecurrenceStartAt: t.RecurrenceStartAt,
		NextDueAt:         t.NextDueAt,
		CreatedBy:         t.CreatedByPublicID,
		ClosedBy:          t.ClosedByPublicID,
		AssignedTo:        t.AssignedToPublicID,
		CreatedAt:         t.CreatedAt,
		UpdatedAt:         t.UpdatedAt,
	}
}

func CreateTaskHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Title       string     `json:"title"`
		Description *string    `json:"description"`
		DueAt       *time.Time `json:"due_at"`
		RepeatEvery *int       `json:"repeat_every"`
		RepeatUnit  *string    `json:"repeat_unit"`
		AssignedTo  *string    `json:"assigned_to"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		projectID, err := apiutil.ParseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		task, err := deps.TaskService.CreateTask(r.Context(), service.CreateTaskParams{
			WorkspaceID: user.WorkspaceID,
			CreatorID:   user.ID,
			ProjectID:   projectID,
			Title:       req.Title,
			Description: req.Description,
			DueAt:       req.DueAt,
			RepeatEvery: req.RepeatEvery,
			RepeatUnit:  req.RepeatUnit,
			AssignedTo:  req.AssignedTo,
		})

		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusCreated, mapTaskToResponse(task, user.WorkspacePublicID))
	}
}

func ListProjectTasksHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		projectID, err := apiutil.ParseInt64Param(r, "projectId")
		if err != nil || projectID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid project id")
			return
		}

		tasks, err := deps.TaskService.ListProjectTasks(r.Context(), user.WorkspaceID, projectID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := make([]TaskResponse, 0, len(tasks))
		for _, t := range tasks {
			resp = append(resp, mapTaskToResponse(t, user.WorkspacePublicID))
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

func GetTaskHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := apiutil.ParseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		task, err := deps.TaskService.GetTask(r.Context(), user.WorkspaceID, taskID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapTaskToResponse(task, user.WorkspacePublicID))
	}
}

func UpdateTaskHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`

		DueAt      *time.Time `json:"due_at"`
		ClearDueAt *bool      `json:"clear_due_at"`

		Completed *bool `json:"completed"`

		RepeatEvery *int    `json:"repeat_every"`
		RepeatUnit  *string `json:"repeat_unit"`
		ClearRepeat *bool   `json:"clear_repeat"`

		AssignedTo      *string `json:"assigned_to"`
		ClearAssignedTo *bool   `json:"clear_assigned_to"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := apiutil.ParseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		var closedBy *int64
		if req.Completed != nil && *req.Completed {
			closedBy = &user.ID
		}

		task, err := deps.TaskService.UpdateTask(r.Context(), user.WorkspaceID, taskID, service.UpdateTaskParams{
			Title:           req.Title,
			Description:     req.Description,
			DueAt:           req.DueAt,
			ClearDueAt:      req.ClearDueAt != nil && *req.ClearDueAt,
			Completed:       req.Completed,
			RepeatEvery:     req.RepeatEvery,
			RepeatUnit:      req.RepeatUnit,
			ClearRepeat:     req.ClearRepeat != nil && *req.ClearRepeat,
			AssignedTo:      req.AssignedTo,
			ClearAssignedTo: req.ClearAssignedTo != nil && *req.ClearAssignedTo,
			ClosedBy:        closedBy,
		})

		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapTaskToResponse(task, user.WorkspacePublicID))
	}
}

func DeleteTaskHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := apiutil.ParseInt64Param(r, "id")
		if err != nil || taskID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		if err := deps.TaskService.DeleteTask(r.Context(), user.WorkspaceID, taskID); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

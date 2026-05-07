package tasks

import (
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
)

type occurrenceResponse struct {
	ID              int64      `json:"id"`
	TaskID          int64      `json:"task_id"`
	OccurrenceIndex int64      `json:"occurrence_index"`
	DueAt           time.Time  `json:"due_at"`
	CompletedAt     *time.Time `json:"completed_at"`
}

func parseTimeQuery(r *http.Request, key string) (*time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func ListTaskOccurrencesHandler(deps app.Deps) http.HandlerFunc {
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

		fromQ, err := parseTimeQuery(r, "from")
		if err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid from (use RFC3339)")
			return
		}
		toQ, err := parseTimeQuery(r, "to")
		if err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid to (use RFC3339)")
			return
		}

		now := time.Now().UTC()
		from := now.AddDate(0, 0, -30)
		to := now.AddDate(0, 0, 120)
		if fromQ != nil {
			from = fromQ.UTC()
		}
		if toQ != nil {
			to = toQ.UTC()
		}
		if !to.After(from) {
			apiutil.WriteErr(w, http.StatusBadRequest, "to must be after from")
			return
		}

		occ, err := deps.TaskService.ListTaskOccurrences(r.Context(), user.WorkspaceID, taskID, from, to)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		out := make([]occurrenceResponse, 0, len(occ))
		for _, o := range occ {
			out = append(out, occurrenceResponse{
				ID:              o.ID,
				TaskID:          o.TaskID,
				OccurrenceIndex: o.OccurrenceIndex,
				DueAt:           o.DueAt,
				CompletedAt:     o.CompletedAt,
			})
		}

		apiutil.WriteJSON(w, http.StatusOK, out)
	}
}

func UpdateTaskOccurrenceHandler(deps app.Deps) http.HandlerFunc {
	type reqBody struct {
		Completed *bool `json:"completed"`
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
		occID, err := apiutil.ParseInt64Param(r, "occurrenceId")
		if err != nil || occID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid occurrence id")
			return
		}

		var req reqBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Completed == nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "completed is required")
			return
		}

		updated, err := deps.TaskService.UpdateTaskOccurrence(r.Context(), user.WorkspaceID, taskID, occID, *req.Completed)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, occurrenceResponse{
			ID:              updated.ID,
			TaskID:          updated.TaskID,
			OccurrenceIndex: updated.OccurrenceIndex,
			DueAt:           updated.DueAt,
			CompletedAt:     updated.CompletedAt,
		})
	}
}

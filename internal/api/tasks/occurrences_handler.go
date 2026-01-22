package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type occurrenceResponse struct {
	ID          int64      `json:"id"`
	TaskID      int64      `json:"task_id"`
	DueAt       time.Time  `json:"due_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

func parseInt64URLParam(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
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

// recurringTaskVisible ensures:
// - task belongs to user
// - task not deleted
// - project not deleted
// - and task is recurring (repeat_* set)
func recurringTaskVisible(ctx context.Context, db *pgxpool.Pool, userID, taskID int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1
		   FROM tasks t
		   JOIN projects p ON p.id = t.project_id
		   WHERE t.id = $1
		     AND (t.user_id = $2 OR EXISTS (SELECT 1 FROM users WHERE id = $2 AND is_admin = true))
		     AND t.deleted_at IS NULL
		     AND p.deleted_at IS NULL
		     AND t.repeat_every IS NOT NULL
		     AND t.repeat_unit IS NOT NULL
		 )`,
		taskID, userID,
	).Scan(&ok)
	return ok, err
}

func ListTaskOccurrencesHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseInt64URLParam(r, "taskId")
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		fromQ, err := parseTimeQuery(r, "from")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid from (use RFC3339)")
			return
		}
		toQ, err := parseTimeQuery(r, "to")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid to (use RFC3339)")
			return
		}

		now := time.Now().UTC()
		from := now.AddDate(0, 0, -30)
		to := now.AddDate(0, 0, 60)
		if fromQ != nil {
			from = fromQ.UTC()
		}
		if toQ != nil {
			to = toQ.UTC()
		}
		if !to.After(from) {
			writeErr(w, http.StatusBadRequest, "to must be after from")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Only recurring tasks have occurrences.
		isRec, err := recurringTaskVisible(ctx, db, user.ID, taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to verify task")
			return
		}
		if !isRec {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}

		// Lazy-generate up to `to` so the range query is complete.
		if err := app.EnsureOccurrencesUpTo(ctx, db, user.ID, taskID, to); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to generate occurrences")
			return
		}

		occ, err := app.ListTaskOccurrences(ctx, db, user.ID, taskID, from, to)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list occurrences")
			return
		}

		out := make([]occurrenceResponse, 0, len(occ))
		for _, o := range occ {
			out = append(out, occurrenceResponse{
				ID:          o.ID,
				TaskID:      o.TaskID,
				DueAt:       o.DueAt,
				CompletedAt: o.CompletedAt,
			})
		}

		writeJSON(w, http.StatusOK, out)
	}
}

/* ---------------- PATCH /tasks/{taskId}/occurrences/{occurrenceId} ---------------- */

func UpdateTaskOccurrenceHandler(db *pgxpool.Pool) http.HandlerFunc {
	type reqBody struct {
		Completed *bool `json:"completed"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseInt64URLParam(r, "taskId")
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}
		occID, err := parseInt64URLParam(r, "occurrenceId")
		if err != nil || occID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid occurrence id")
			return
		}

		var req reqBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Completed == nil {
			writeErr(w, http.StatusBadRequest, "completed is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Ensure this task is a visible recurring task.
		isRec, err := recurringTaskVisible(ctx, db, user.ID, taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to verify task")
			return
		}
		if !isRec {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}

		updated, err := app.SetOccurrenceCompleted(ctx, db, user.ID, taskID, occID, *req.Completed)
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "occurrence not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update occurrence")
			return
		}

		// Best-effort: keep next_due_at fresh without cron
		_ = app.EnsureOccurrencesUpTo(ctx, db, user.ID, taskID, time.Now().UTC().AddDate(0, 0, 60))

		writeJSON(w, http.StatusOK, occurrenceResponse{
			ID:          updated.ID,
			TaskID:      updated.TaskID,
			DueAt:       updated.DueAt,
			CompletedAt: updated.CompletedAt,
		})
	}
}

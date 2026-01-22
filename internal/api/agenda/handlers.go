package agenda

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5/pgxpool"
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

type Item struct {
	Kind         string `json:"kind"` // "task" | "occurrence"
	TaskID       int64  `json:"task_id"`
	OccurrenceID *int64 `json:"occurrence_id,omitempty"`

	ProjectID int64  `json:"project_id"`
	Title     string `json:"title"`
	DueAt     string `json:"due_at"` // RFC3339
}

func parseRFC3339Query(r *http.Request, key string) (*time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	u := t.UTC()
	return &u, nil
}

func GetAgendaHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		fromQ, err := parseRFC3339Query(r, "from")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid from (use RFC3339)")
			return
		}
		toQ, err := parseRFC3339Query(r, "to")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid to (use RFC3339)")
			return
		}

		now := time.Now().UTC()
		from := now.AddDate(0, 0, -1)
		to := now.AddDate(0, 0, 7)
		if fromQ != nil {
			from = *fromQ
		}
		if toQ != nil {
			to = *toQ
		}

		if !to.After(from) {
			writeErr(w, http.StatusBadRequest, "to must be after from")
			return
		}

		// Safety cap: don’t allow huge windows (can be adjusted later)
		if to.Sub(from) > 180*24*time.Hour {
			writeErr(w, http.StatusBadRequest, "time window too large (max 180 days)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// 1) Find all visible recurring tasks and ensure occurrences up to `to`.
		rows, err := db.Query(ctx,
			`SELECT t.id
			 FROM tasks t
			 JOIN projects p ON p.id = t.project_id
			 WHERE t.user_id = $1
			   AND t.deleted_at IS NULL
			   AND p.deleted_at IS NULL
			   AND t.repeat_every IS NOT NULL
			   AND t.repeat_unit IS NOT NULL`,
			user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to load recurring tasks")
			return
		}
		defer rows.Close()

		var recurringIDs []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read recurring tasks")
				return
			}
			recurringIDs = append(recurringIDs, id)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read recurring tasks")
			return
		}

		for _, taskID := range recurringIDs {
			if err := app.EnsureOccurrencesUpTo(ctx, db, user.ID, taskID, to); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to generate occurrences")
				return
			}
		}

		// 2) Query due items in range:
		//    - non-recurring tasks due in [from,to], incomplete
		//    - recurring occurrences due in [from,to], incomplete
		rows2, err := db.Query(ctx,
			`SELECT kind, task_id, occurrence_id, project_id, title, due_at
			 FROM (
			   -- Non-recurring tasks
			   SELECT
			     'task'::text AS kind,
			     t.id          AS task_id,
			     NULL::bigint  AS occurrence_id,
			     t.project_id  AS project_id,
			     t.title       AS title,
			     t.due_at      AS due_at
			   FROM tasks t
			   JOIN projects p ON p.id = t.project_id
			   WHERE t.user_id = $1
			     AND t.deleted_at IS NULL
			     AND p.deleted_at IS NULL
			     AND t.repeat_every IS NULL
			     AND t.repeat_unit IS NULL
			     AND t.completed_at IS NULL
			     AND t.due_at IS NOT NULL
			     AND t.due_at >= $2 AND t.due_at <= $3

			   UNION ALL

			   -- Recurring occurrences
			   SELECT
			     'occurrence'::text AS kind,
			     t.id               AS task_id,
			     o.id               AS occurrence_id,
			     t.project_id       AS project_id,
			     t.title            AS title,
			     o.due_at           AS due_at
			   FROM task_occurrences o
			   JOIN tasks t ON t.id = o.task_id
			   JOIN projects p ON p.id = t.project_id
			   WHERE o.user_id = $1
			     AND t.deleted_at IS NULL
			     AND p.deleted_at IS NULL
			     AND o.completed_at IS NULL
			     AND o.due_at >= $2 AND o.due_at <= $3
			 ) x
			 ORDER BY due_at, task_id, occurrence_id NULLS FIRST`,
			user.ID, from, to,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to build agenda")
			return
		}
		defer rows2.Close()

		out := make([]Item, 0, 128)
		for rows2.Next() {
			var (
				kind      string
				taskID    int64
				occID     *int64
				projectID int64
				title     string
				dueAt     time.Time
			)

			if err := rows2.Scan(&kind, &taskID, &occID, &projectID, &title, &dueAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read agenda")
				return
			}

			out = append(out, Item{
				Kind:         kind,
				TaskID:       taskID,
				OccurrenceID: occID,
				ProjectID:    projectID,
				Title:        title,
				DueAt:        dueAt.UTC().Format(time.RFC3339),
			})
		}
		if err := rows2.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read agenda")
			return
		}

		writeJSON(w, http.StatusOK, out)
	}
}

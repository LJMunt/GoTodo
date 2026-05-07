package agenda

import (
	"net/http"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
)

type Item struct {
	Type            string     `json:"type"` // "task" or "occurrence"
	ID              int64      `json:"id"`
	TaskID          *int64     `json:"task_id,omitempty"`
	OccurrenceIndex *int64     `json:"occurrence_index,omitempty"`
	Title           string     `json:"title"`
	DueAt           time.Time  `json:"due_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
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
	return &t, nil
}

func GetAgendaHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		fromQ, err := parseRFC3339Query(r, "from")
		if err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid from (use RFC3339)")
			return
		}
		toQ, err := parseRFC3339Query(r, "to")
		if err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid to (use RFC3339)")
			return
		}

		now := time.Now().UTC()
		from := now.AddDate(0, 0, -7)
		to := now.AddDate(0, 0, 30)
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

		ctx := r.Context()

		// 1. Fetch recurring tasks that might have occurrences in [from, to]
		// and ensure they are generated.
		rows, err := deps.DB.Query(ctx,
			`SELECT id FROM tasks 
			 WHERE workspace_id=$1 AND repeat_every IS NOT NULL AND deleted_at IS NULL`,
			user.WorkspaceID,
		)
		if err == nil {
			var taskIDs []int64
			for rows.Next() {
				var tid int64
				if err := rows.Scan(&tid); err == nil {
					taskIDs = append(taskIDs, tid)
				}
			}
			rows.Close()

			for _, tid := range taskIDs {
				_ = deps.TaskService.EnsureOccurrencesUpTo(ctx, deps.DB, user.WorkspaceID, tid, to)
			}
		}

		// 2. Query non-recurring tasks in range
		items := make([]Item, 0, 100)

		rows, err = deps.DB.Query(ctx,
			`SELECT id, title, due_at, completed_at
			 FROM tasks
			 WHERE workspace_id=$1 AND repeat_every IS NULL AND deleted_at IS NULL
			   AND due_at >= $2 AND due_at <= $3`,
			user.WorkspaceID, from, to,
		)
		if err == nil {
			for rows.Next() {
				var i Item
				i.Type = "task"
				if err := rows.Scan(&i.ID, &i.Title, &i.DueAt, &i.CompletedAt); err == nil {
					items = append(items, i)
				}
			}
			rows.Close()
		}

		// 3. Query occurrences in range
		rows, err = deps.DB.Query(ctx,
			`SELECT o.id, o.task_id, o.occurrence_index, o.due_at, o.completed_at, t.title
			 FROM task_occurrences o
			 JOIN tasks t ON t.id = o.task_id
			 WHERE o.workspace_id=$1 AND t.deleted_at IS NULL
			   AND o.due_at >= $2 AND o.due_at <= $3`,
			user.WorkspaceID, from, to,
		)
		if err == nil {
			for rows.Next() {
				var i Item
				i.Type = "occurrence"
				if err := rows.Scan(&i.ID, &i.TaskID, &i.OccurrenceIndex, &i.DueAt, &i.CompletedAt, &i.Title); err == nil {
					items = append(items, i)
				}
			}
			rows.Close()
		}

		apiutil.WriteJSON(w, http.StatusOK, items)
	}
}

package tasks

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskTagResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func parseTaskID(r *http.Request) (int64, error) {
	// route param name is {taskId}
	return parseInt64Param(r, "taskId")
}

// Checks task visibility: task not deleted and project not deleted.
func taskVisible(ctx context.Context, db *pgxpool.Pool, userID, taskID int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1
		   FROM tasks t
		   JOIN projects p ON p.id = t.project_id
		   WHERE t.id = $1
		     AND t.user_id = $2
		     AND t.deleted_at IS NULL
		     AND p.deleted_at IS NULL
		 )`,
		taskID, userID,
	).Scan(&ok)
	return ok, err
}

func GetTaskTagsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseTaskID(r)
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		ok, err = taskVisible(ctx, db, user.ID, taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to verify task")
			return
		}
		if !ok {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}

		rows, err := db.Query(ctx,
			`SELECT tg.id, tg.name
									 FROM task_tags tt
									 JOIN tags tg ON tg.id = tt.tag_id
									 WHERE tt.user_id = $1 AND tt.task_id = $2
									 ORDER BY tg.name , tg.id `,
			user.ID, taskID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list task tags")
			return
		}
		defer rows.Close()

		out := make([]TaskTagResponse, 0, 16)
		for rows.Next() {
			var t TaskTagResponse
			if err := rows.Scan(&t.ID, &t.Name); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read task tags")
				return
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read task tags")
			return
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func PutTaskTagsHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		TagIDs []int64 `json:"tag_ids"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		taskID, err := parseTaskID(r)
		if err != nil || taskID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid task id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.TagIDs == nil {
			writeErr(w, http.StatusBadRequest, "tag_ids is required")
			return
		}

		// De-dupe + validate positive IDs
		seen := make(map[int64]struct{}, len(req.TagIDs))
		tagIDs := make([]int64, 0, len(req.TagIDs))
		for _, id := range req.TagIDs {
			if id <= 0 {
				writeErr(w, http.StatusBadRequest, "tag_ids must be positive integers")
				return
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			tagIDs = append(tagIDs, id)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		ok, err = taskVisible(ctx, db, user.ID, taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to verify task")
			return
		}
		if !ok {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Validate that all tag IDs exist and belong to this user
		if len(tagIDs) > 0 {
			var count int
			err = tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM tags WHERE user_id=$1 AND id = ANY($2::bigint[])`,
				user.ID, tagIDs,
			).Scan(&count)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to validate tags")
				return
			}
			if count != len(tagIDs) {
				writeErr(w, http.StatusBadRequest, "one or more tag_ids are invalid")
				return
			}
		}

		// Replace-all:
		_, err = tx.Exec(ctx,
			`DELETE FROM task_tags WHERE user_id=$1 AND task_id=$2`,
			user.ID, taskID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update task tags")
			return
		}

		if len(tagIDs) > 0 {
			_, err = tx.Exec(ctx,
				`INSERT INTO task_tags (user_id, task_id, tag_id)
				 SELECT $1, $2, unnest($3::bigint[])
				 ON CONFLICT DO NOTHING`,
				user.ID, taskID, tagIDs,
			)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to update task tags")
				return
			}
		}

		rows, err := tx.Query(ctx,
			`SELECT tg.id, tg.name
									 FROM task_tags tt
									 JOIN tags tg ON tg.id = tt.tag_id
									 WHERE tt.user_id = $1 AND tt.task_id = $2
									 ORDER BY tg.name , tg.id `,
			user.ID, taskID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read task tags")
			return
		}
		defer rows.Close()

		out := make([]TaskTagResponse, 0, 16)
		for rows.Next() {
			var t TaskTagResponse
			if err := rows.Scan(&t.ID, &t.Name); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read task tags")
				return
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read task tags")
			return
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to commit task tags update")
			return
		}

		writeJSON(w, http.StatusOK, out)
	}
}

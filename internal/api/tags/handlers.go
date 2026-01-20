package tags

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	authmw "GoToDo/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TagResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

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

func parseTagID(r *http.Request) (int64, error) {
	s := chi.URLParam(r, "tagId")
	return strconv.ParseInt(s, 10, 64)
}

func normalizeTagName(s string) string {
	return strings.TrimSpace(s)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func ListTagsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		q := strings.TrimSpace(r.URL.Query().Get("q"))

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `SELECT id, name, created_at, updated_at FROM tags WHERE user_id = $1`
		args := []any{user.ID}

		if q != "" {
			// citext makes name comparisons case-insensitive
			query += ` AND name ILIKE '%' || $2 || '%'`
			args = append(args, q)
		}

		query += ` ORDER BY name ASC, id ASC`

		rows, err := db.Query(ctx, query, args...)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list tags")
			return
		}
		defer rows.Close()

		out := make([]TagResponse, 0, 64)
		for rows.Next() {
			var t TagResponse
			if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read tags")
				return
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read tags")
			return
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func CreateTagHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		name := normalizeTagName(req.Name)
		if name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}
		if len(name) > 64 {
			writeErr(w, http.StatusBadRequest, "name too long (max 64)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var out TagResponse
		err := db.QueryRow(ctx,
			`INSERT INTO tags (user_id, name)
			 VALUES ($1, $2)
			 RETURNING id, name, created_at, updated_at`,
			user.ID, name,
		).Scan(&out.ID, &out.Name, &out.CreatedAt, &out.UpdatedAt)

		if err != nil {
			if isUniqueViolation(err) {
				writeErr(w, http.StatusConflict, "tag already exists")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to create tag")
			return
		}

		writeJSON(w, http.StatusCreated, out)
	}
}

func RenameTagHandler(db *pgxpool.Pool) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		tagID, err := parseTagID(r)
		if err != nil || tagID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid tag id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		name := normalizeTagName(req.Name)
		if name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}
		if len(name) > 64 {
			writeErr(w, http.StatusBadRequest, "name too long (max 64)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var out TagResponse
		err = db.QueryRow(ctx,
			`UPDATE tags
			 SET name = $1, updated_at = now()
			 WHERE id = $2 AND user_id = $3
			 RETURNING id, name, created_at, updated_at`,
			name, tagID, user.ID,
		).Scan(&out.ID, &out.Name, &out.CreatedAt, &out.UpdatedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "tag not found")
			return
		}
		if err != nil {
			if isUniqueViolation(err) {
				writeErr(w, http.StatusConflict, "tag already exists")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to rename tag")
			return
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func DeleteTagHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		tagID, err := parseTagID(r)
		if err != nil || tagID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid tag id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`DELETE FROM tags WHERE id = $1 AND user_id = $2`,
			tagID, user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete tag")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "tag not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

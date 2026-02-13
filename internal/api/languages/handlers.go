package languages

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type languageDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type AdminLanguage struct {
	Code      string    `json:"code"`
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

// ListLanguagesHandler (public): GET /api/v1/lang
func ListLanguagesHandler(db languageDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, "SELECT code, name FROM languages ORDER BY code ASC")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch languages")
			return
		}
		defer rows.Close()

		langs := []Language{}
		for rows.Next() {
			var l Language
			if err := rows.Scan(&l.Code, &l.Name); err != nil {
				continue
			}
			langs = append(langs, l)
		}

		writeJSON(w, http.StatusOK, langs)
	}
}

// AdminListLanguagesHandler (admin): GET /api/v1/admin/lang
func AdminListLanguagesHandler(db languageDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, "SELECT code, name, created_at, updated_at FROM languages ORDER BY code ASC")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch languages")
			return
		}
		defer rows.Close()

		langs := []AdminLanguage{}
		for rows.Next() {
			var l AdminLanguage
			if err := rows.Scan(&l.Code, &l.Name, &l.CreatedAt, &l.UpdatedAt); err != nil {
				continue
			}
			langs = append(langs, l)
		}

		writeJSON(w, http.StatusOK, langs)
	}
}

var langCodeRegex = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

// AdminCreateLanguageHandler (admin): POST /api/v1/admin/lang
func AdminCreateLanguageHandler(db languageDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Language
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Code == "" || req.Name == "" {
			writeErr(w, http.StatusBadRequest, "code and name are required")
			return
		}

		if !langCodeRegex.MatchString(req.Code) {
			writeErr(w, http.StatusBadRequest, "code must be 2 letters or in format xx-xx (e.g., en-gb)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var l AdminLanguage
		err := db.QueryRow(ctx, `
			INSERT INTO languages (code, name)
			VALUES ($1, $2)
			RETURNING code, name, created_at, updated_at
		`, req.Code, req.Name).Scan(&l.Code, &l.Name, &l.CreatedAt, &l.UpdatedAt)

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
				writeErr(w, http.StatusConflict, "language code already exists")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to create language")
			return
		}

		writeJSON(w, http.StatusCreated, l)
	}
}

// AdminDeleteLanguageHandler (admin): DELETE /api/v1/admin/lang/{code}
func AdminDeleteLanguageHandler(db languageDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := chi.URLParam(r, "code")
		if code == "" {
			writeErr(w, http.StatusBadRequest, "language code is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Requirement: cannot delete default language
		var defaultLang string
		err := db.QueryRow(ctx, "SELECT value_json FROM config_keys WHERE key = 'defaults.userLanguage'").Scan(&defaultLang)
		if err == nil {
			// Strip quotes from JSON string
			defaultLang = strings.Trim(defaultLang, "\"")
			if code == defaultLang {
				writeErr(w, http.StatusConflict, "cannot delete default language")
				return
			}
		}

		// Also prevent deleting 'en' if it's currently hardcoded in any secondary fallbacks
		// but since we made the trigger dynamic, this is less critical.
		// However, for consistency with the system's "original" intent, we might want to keep one base language.
		// If the user wants to remove 'en' entirely, they should be able to as long as it's not the default.

		// Requirement: cannot delete last active language
		var count int
		err = db.QueryRow(ctx, "SELECT COUNT(*) FROM languages").Scan(&count)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to check language count")
			return
		}
		if count <= 1 {
			writeErr(w, http.StatusConflict, "cannot delete last active language")
			return
		}

		tag, err := db.Exec(ctx, "DELETE FROM languages WHERE code = $1", code)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete language")
			return
		}

		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "language not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

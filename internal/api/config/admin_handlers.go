package config

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Key struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	DataType    string `json:"data_type"`
	IsPublic    bool   `json:"is_public"`
}

type Translations map[string]string

// ListConfigKeysHandler returns all configuration keys and metadata.
func ListConfigKeysHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, `
			SELECT key, description, data_type, is_public
			FROM config_keys
			ORDER BY key ASC
		`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch config keys")
			return
		}
		defer rows.Close()

		var keys []Key
		for rows.Next() {
			var k Key
			var desc *string
			if err := rows.Scan(&k.Key, &desc, &k.DataType, &k.IsPublic); err != nil {
				continue
			}
			if desc != nil {
				k.Description = *desc
			}
			keys = append(keys, k)
		}

		writeJSON(w, http.StatusOK, keys)
	}
}

// GetTranslationsHandler returns all translations for a specific language.
func GetTranslationsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			writeErr(w, http.StatusBadRequest, "lang parameter is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, `
			SELECT key, value
			FROM config_translations
			WHERE language_code = $1
		`, lang)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch translations")
			return
		}
		defer rows.Close()

		translations := make(Translations)
		for rows.Next() {
			var key, val string
			if err := rows.Scan(&key, &val); err != nil {
				continue
			}
			translations[key] = val
		}

		writeJSON(w, http.StatusOK, translations)
	}
}

// UpdateTranslationsHandler bulk updates/upserts translations for a language.
func UpdateTranslationsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			writeErr(w, http.StatusBadRequest, "lang parameter is required")
			return
		}

		var translations Translations
		if err := json.NewDecoder(r.Body).Decode(&translations); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer tx.Rollback(ctx)

		for key, val := range translations {
			_, err := tx.Exec(ctx, `
				INSERT INTO config_translations (key, language_code, value, updated_at)
				VALUES ($1, $2, $3, NOW())
				ON CONFLICT (key, language_code) DO UPDATE SET
					value = EXCLUDED.value,
					updated_at = NOW()
			`, key, lang, val)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to update translation")
				return
			}
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

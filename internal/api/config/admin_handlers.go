package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"GoToDo/internal/logging"
	"GoToDo/internal/secrets"
)

type Key struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	DataType    string `json:"data_type"`
	IsPublic    bool   `json:"is_public"`
}

type Translations map[string]string

type ConfigValues map[string]any

// ListConfigKeysHandler returns all configuration keys and metadata.
func ListConfigKeysHandler(db configQuerier) http.HandlerFunc {
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
func GetTranslationsHandler(db configQuerier) http.HandlerFunc {
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

// GetConfigValuesHandler returns all backend JSON config values (non-string keys only).
func GetConfigValuesHandler(db configQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, `
			SELECT key, value_json, is_secret
			FROM config_keys
			WHERE value_json IS NOT NULL
			ORDER BY key ASC
		`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch config values")
			return
		}
		defer rows.Close()

		values := make(map[string]any)
		for rows.Next() {
			var key string
			var raw any
			var isSecret bool
			if err := rows.Scan(&key, &raw, &isSecret); err != nil {
				continue
			}
			// pgx decodes jsonb into []byte or map depending on settings; marshal/unmarshal for safety
			b, mErr := json.Marshal(raw)
			if mErr != nil {
				continue
			}
			var v any
			if uErr := json.Unmarshal(b, &v); uErr != nil {
				continue
			}
			if isSecret {
				values[key] = ""
				continue
			}
			values[key] = v
		}

		writeJSON(w, http.StatusOK, values)
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

		l := logging.From(r.Context())
		l.Info().Str("lang", lang).Int("count", len(translations)).Msg("updating translations")

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
			l.Error().Err(err).Str("lang", lang).Msg("failed to commit translations update")
			writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
			return
		}

		l.Info().Str("lang", lang).Msg("translations updated successfully")
		w.WriteHeader(http.StatusNoContent)
	}
}

// UpdateConfigValuesHandler bulk upserts backend JSON config values for non-string keys.
// Rules:
// - Reject updates for keys with data_type = 'string' (must go through translations)
// - Validate JSON type matches data_type for 'boolean' and 'number'
// - (Optional) Reject if is_public = true to avoid public non-string keys
func UpdateConfigValuesHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload ConfigValues
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		l := logging.From(r.Context())
		l.Info().Int("count", len(payload)).Msg("bulk configuration update initiated")

		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer tx.Rollback(ctx)

		validateType := func(dataType string, v any) error {
			switch dataType {
			case "boolean":
				if _, ok := v.(bool); !ok {
					return errors.New("value must be a boolean for data_type=boolean")
				}
			case "number":
				// JSON numbers decode to float64 in Go by default
				if _, ok := v.(float64); !ok {
					return errors.New("value must be a number for data_type=number")
				}
			}
			return nil
		}

		for key, val := range payload {
			var dataType string
			var isPublic bool
			var isSecret bool
			// Ensure key exists and fetch metadata
			if err := tx.QueryRow(ctx, `SELECT data_type, is_public, is_secret FROM config_keys WHERE key = $1`, key).Scan(&dataType, &isPublic, &isSecret); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown config key: %s", key))
					return
				}
				writeErr(w, http.StatusInternalServerError, "failed to validate key")
				return
			}

			if dataType == "string" && isPublic {
				writeErr(w, http.StatusBadRequest, "string keys must be updated via translations endpoint")
				return
			}

			if isPublic {
				writeErr(w, http.StatusBadRequest, "cannot set backend value for public keys")
				return
			}

			if err := validateType(dataType, val); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}

			// Marshal value to JSON for storage; if nil (explicit null), set to NULL
			if val == nil {
				if _, err := tx.Exec(ctx, `UPDATE config_keys SET value_json = NULL, updated_at = NOW() WHERE key = $1`, key); err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to update value")
					return
				}
				continue
			}

			if isSecret {
				s, ok := val.(string)
				if !ok {
					writeErr(w, http.StatusBadRequest, "secret config value must be a string")
					return
				}
				mk, err := secrets.LoadMasterKey()
				if err != nil {
					writeErr(w, http.StatusInternalServerError, "secrets master key not configured")
					return
				}
				ct, err := secrets.EncryptString(strings.TrimSpace(s), mk, []byte(key))
				if err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to encrypt secret config value")
					return
				}
				b, _ := json.Marshal(ct)
				if _, err := tx.Exec(ctx, `UPDATE config_keys SET value_json = $2::jsonb, updated_at = NOW() WHERE key = $1`, key, string(b)); err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to update value")
					return
				}
				continue
			}

			b, err := json.Marshal(val)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "invalid JSON value")
				return
			}
			if _, err := tx.Exec(ctx, `UPDATE config_keys SET value_json = $2::jsonb, updated_at = NOW() WHERE key = $1`, key, string(b)); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to update value")
				return
			}
		}

		if err := tx.Commit(ctx); err != nil {
			l.Error().Err(err).Msg("failed to commit configuration update")
			writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
			return
		}

		l.Info().Int("keys_updated", len(payload)).Msg("bulk configuration update successful")
		w.WriteHeader(http.StatusNoContent)
	}
}

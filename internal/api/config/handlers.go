package config

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// GetConfigHandler returns the public configuration as a nested JSON object.
func GetConfigHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			// Basic language negotiation from Accept-Language header
			acceptLang := r.Header.Get("Accept-Language")
			if acceptLang != "" {
				parts := strings.Split(acceptLang, ",")
				lang = strings.TrimSpace(strings.Split(parts[0], ";")[0])
			}
		}

		if lang == "" {
			lang = "en"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		rows, err := db.Query(ctx, `
			SELECT ck.key, ck.data_type, COALESCE(ct.value, ct_en.value, '') as value
			FROM config_keys ck
			LEFT JOIN config_translations ct ON ck.key = ct.key AND ct.language_code = $1
			LEFT JOIN config_translations ct_en ON ck.key = ct_en.key AND ct_en.language_code = 'en'
			WHERE ck.is_public = true
		`, lang)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch configuration")
			return
		}
		defer rows.Close()

		flatConfig := make(map[string]any)
		for rows.Next() {
			var key, dataType, val string
			if err := rows.Scan(&key, &dataType, &val); err != nil {
				continue
			}
			flatConfig[key] = castConfigValue(val, dataType)
		}

		nestedConfig := NestConfig(flatConfig)

		// Set Cache-Control header for stateless scaling/CDN
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, nestedConfig)
	}
}

// castConfigValue converts the string value from DB to the appropriate Go type based on data_type.
func castConfigValue(val string, dataType string) any {
	switch dataType {
	case "boolean":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false
		}
		return b
	case "number":
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0.0
		}
		return n
	default:
		return val
	}
}

// NestConfig converts flat dot-notation keys into a nested map structure.
func NestConfig(flat map[string]any) map[string]any {
	nested := make(map[string]any)
	for k, v := range flat {
		parts := strings.Split(k, ".")
		curr := nested
		for i, part := range parts {
			if i == len(parts)-1 {
				curr[part] = v
			} else {
				if _, ok := curr[part]; !ok {
					curr[part] = make(map[string]any)
				}
				// Ensure it's a map before proceeding
				if nextMap, ok := curr[part].(map[string]any); ok {
					curr = nextMap
				} else {
					break
				}
			}
		}
	}
	return nested
}

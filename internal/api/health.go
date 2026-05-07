package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/repository"
	"github.com/jackc/pgx/v5"
)

const Version = "v0.7.0"

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

func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}
}

func VersionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"version": Version})
	}
}

func ReadyHandler(db repository.DBTX) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeErr(w, http.StatusServiceUnavailable, "db connection not initialized")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var one int
		if err := db.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
			if err == pgx.ErrNoRows {
				writeErr(w, http.StatusServiceUnavailable, "db not ready")
				return
			}
			writeErr(w, http.StatusServiceUnavailable, "db not ready")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ready": true})
	}
}

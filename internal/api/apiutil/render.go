package apiutil

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"GoToDo/internal/service"

	"github.com/go-chi/chi/v5"
)

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteErr(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, APIError{Error: msg})
}

func WriteErrDetailed(w http.ResponseWriter, status int, err, msg string) {
	WriteJSON(w, status, APIError{Error: err, Message: msg})
}

func ParseInt64Param(r *http.Request, key string) (int64, error) {
	s := chi.URLParam(r, key)
	return strconv.ParseInt(s, 10, 64)
}

func HandleServiceErr(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrNotFound) {
		WriteErr(w, http.StatusNotFound, err.Error())
	} else if errors.Is(err, service.ErrForbidden) {
		WriteErr(w, http.StatusForbidden, err.Error())
	} else if errors.Is(err, service.ErrInvalidInput) {
		WriteErr(w, http.StatusBadRequest, err.Error())
	} else if errors.Is(err, service.ErrUnauthorized) {
		WriteErr(w, http.StatusUnauthorized, err.Error())
	} else {
		WriteErr(w, http.StatusInternalServerError, "internal server error")
	}
}

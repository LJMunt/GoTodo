package users

import (
	"encoding/json"
	"net/http"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/service"
)

type userSettings struct {
	Theme                string `json:"theme"`
	ShowCompletedDefault bool   `json:"showCompletedDefault"`
	Language             string `json:"language"`
}

type WorkspaceResponse struct {
	PublicID string `json:"public_id"`
	Type     string `json:"type"`
}

type userMeResponse struct {
	PublicID        string              `json:"public_id"`
	Email           string              `json:"email"`
	IsAdmin         bool                `json:"is_admin"`
	IsActive        bool                `json:"is_active"`
	LastLogin       *time.Time          `json:"last_login"`
	EmailVerifiedAt *time.Time          `json:"email_verified_at"`
	Settings        userSettings        `json:"settings"`
	MfaEnabled      bool                `json:"mfa_enabled"`
	Workspaces      []WorkspaceResponse `json:"workspaces"`
}

func MeHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		u, ws, err := deps.UserService.GetUserMe(r.Context(), user.ID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := userMeResponse{
			PublicID:        u.PublicID,
			Email:           u.Email,
			IsAdmin:         u.IsAdmin,
			IsActive:        u.IsActive,
			LastLogin:       u.LastLogin,
			EmailVerifiedAt: u.EmailVerifiedAt,
			Settings: userSettings{
				Theme:                u.UITheme,
				ShowCompletedDefault: u.ShowCompletedDefault,
				Language:             u.Language,
			},
			MfaEnabled: u.TOTPEnabled,
			Workspaces: []WorkspaceResponse{
				{
					PublicID: ws.PublicID,
					Type:     string(ws.Type),
				},
			},
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

func UpdateMeHandler(deps app.Deps) http.HandlerFunc {
	type settingsRequest struct {
		Theme                *string `json:"theme"`
		ShowCompletedDefault *bool   `json:"showCompletedDefault"`
		Language             *string `json:"language"`
	}
	type request struct {
		Email    *string          `json:"email"`
		Settings *settingsRequest `json:"settings"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		params := service.UpdateUserParams{
			Email: req.Email,
		}
		if req.Settings != nil {
			params.UITheme = req.Settings.Theme
			params.ShowCompletedDefault = req.Settings.ShowCompletedDefault
			params.Language = req.Settings.Language
		}

		u, err := deps.UserService.UpdateUser(r.Context(), user.ID, params)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		_, personalWs, err := deps.UserService.GetUserMe(r.Context(), user.ID)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := userMeResponse{
			PublicID:        u.PublicID,
			Email:           u.Email,
			IsAdmin:         u.IsAdmin,
			IsActive:        u.IsActive,
			LastLogin:       u.LastLogin,
			EmailVerifiedAt: u.EmailVerifiedAt,
			Settings: userSettings{
				Theme:                u.UITheme,
				ShowCompletedDefault: u.ShowCompletedDefault,
				Language:             u.Language,
			},
			MfaEnabled: u.TOTPEnabled,
			Workspaces: []WorkspaceResponse{
				{
					PublicID: personalWs.PublicID,
					Type:     string(personalWs.Type),
				},
			},
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

func DeleteMeHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if err := deps.UserService.DeleteUser(r.Context(), user.ID); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func SearchUsersHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if len(q) < 2 {
			apiutil.WriteJSON(w, http.StatusOK, []any{})
			return
		}

		users, err := deps.UserService.SearchUsers(r.Context(), q)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		type searchResponse struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}

		resp := make([]searchResponse, 0, len(users))
		for _, u := range users {
			resp = append(resp, searchResponse{
				ID:    u.PublicID,
				Email: u.Email,
			})
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

package tags

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"GoToDo/internal/api/apiutil"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/models"
)

type TagResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var allowedColors = map[string]bool{
	"slate":   true,
	"gray":    true,
	"red":     true,
	"orange":  true,
	"amber":   true,
	"yellow":  true,
	"lime":    true,
	"green":   true,
	"emerald": true,
	"teal":    true,
	"cyan":    true,
	"sky":     true,
	"blue":    true,
	"indigo":  true,
	"violet":  true,
	"purple":  true,
	"pink":    true,
}

func isValidColor(c string) bool {
	return allowedColors[c]
}

func mapTagToResponse(t *models.Tag) TagResponse {
	return TagResponse{
		ID:        t.ID,
		Name:      t.Name,
		Color:     t.Color,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func ListTagsHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		q := strings.TrimSpace(r.URL.Query().Get("q"))
		tags, err := deps.TagService.ListTags(r.Context(), user.WorkspaceID, q)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		resp := make([]TagResponse, 0, len(tags))
		for _, t := range tags {
			resp = append(resp, mapTagToResponse(t))
		}

		apiutil.WriteJSON(w, http.StatusOK, resp)
	}
}

func CreateTagHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Name  string `json:"name"`
		Color string `json:"color"`
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

		color := req.Color
		if color == "" {
			color = "slate"
		}
		if !isValidColor(color) {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid color")
			return
		}

		t, err := deps.TagService.CreateTag(r.Context(), user.WorkspaceID, req.Name, color)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusCreated, mapTagToResponse(t))
	}
}

func RenameTagHandler(deps app.Deps) http.HandlerFunc {
	type request struct {
		Name  *string `json:"name"`
		Color *string `json:"color"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		tagID, err := apiutil.ParseInt64Param(r, "tagId")
		if err != nil || tagID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid tag id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Color != nil && !isValidColor(*req.Color) {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid color")
			return
		}

		t, err := deps.TagService.UpdateTag(r.Context(), user.WorkspaceID, tagID, req.Name, req.Color)
		if err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, mapTagToResponse(t))
	}
}

func DeleteTagHandler(deps app.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			apiutil.WriteErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		tagID, err := apiutil.ParseInt64Param(r, "tagId")
		if err != nil || tagID <= 0 {
			apiutil.WriteErr(w, http.StatusBadRequest, "invalid tag id")
			return
		}

		if err := deps.TagService.DeleteTag(r.Context(), user.WorkspaceID, tagID); err != nil {
			apiutil.HandleServiceErr(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

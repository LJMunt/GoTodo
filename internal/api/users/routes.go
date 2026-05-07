package users

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Get("/me", MeHandler(deps))
	r.Patch("/me", UpdateMeHandler(deps))
	r.Delete("/me", DeleteMeHandler(deps))
	r.Get("/search", SearchUsersHandler(deps))
}

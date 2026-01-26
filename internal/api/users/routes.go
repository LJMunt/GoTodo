package users

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Get("/me", MeHandler(deps.DB))
	r.Patch("/me", UpdateMeHandler(deps.DB))
	r.Delete("/me", DeleteMeHandler(deps.DB))
}

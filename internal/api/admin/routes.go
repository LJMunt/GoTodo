package admin

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Route("/users", func(r chi.Router) {
		r.Get("/", ListUsersHandler(deps.DB))
		r.Get("/{id}", GetUserHandler(deps.DB))
		r.Patch("/{id}", UpdateUserHandler(deps.DB))
		r.Delete("/{id}", DeleteUserHandler(deps.DB))
	})
}

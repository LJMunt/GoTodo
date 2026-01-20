package projects

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Route("/", func(r chi.Router) {
		r.Post("/", CreateProjectHandler(deps.DB))
		r.Get("/", ListProjectsHandler(deps.DB))
		r.Get("/{id}", GetProjectHandler(deps.DB))
		r.Patch("/{id}", UpdateProjectHandler(deps.DB))
		r.Delete("/{id}", DeleteProjectHandler(deps.DB))
	})
}

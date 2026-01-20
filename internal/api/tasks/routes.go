package tasks

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	// Project-scoped
	r.Route("/projects/{projectId}/tasks", func(r chi.Router) {
		r.Post("/", CreateTaskHandler(deps.DB))
		r.Get("/", ListProjectTasksHandler(deps.DB))
	})

	// Global task endpoints
	r.Route("/tasks", func(r chi.Router) {
		r.Get("/{id}", GetTaskHandler(deps.DB))
		r.Patch("/{id}", UpdateTaskHandler(deps.DB))
		r.Delete("/{id}", DeleteTaskHandler(deps.DB))

		r.Get("/{taskId}/tags", GetTaskTagsHandler(deps.DB))
		r.Put("/{taskId}/tags", PutTaskTagsHandler(deps.DB))
	})
}

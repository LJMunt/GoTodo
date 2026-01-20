package projects

import (
	"GoToDo/internal/api/tasks"
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Post("/", CreateProjectHandler(deps.DB))
	r.Get("/", ListProjectsHandler(deps.DB))
	r.Get("/{id}", GetProjectHandler(deps.DB))
	r.Patch("/{id}", UpdateProjectHandler(deps.DB))
	r.Delete("/{id}", DeleteProjectHandler(deps.DB))

	r.Route("/{projectId}/tasks", func(r chi.Router) {
		r.Post("/", tasks.CreateTaskHandler(deps.DB))
		r.Get("/", tasks.ListProjectTasksHandler(deps.DB))
	})
}

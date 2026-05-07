package projects

import (
	"GoToDo/internal/api/tasks"
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Post("/", CreateProjectHandler(deps))
	r.Get("/", ListProjectsHandler(deps))
	r.Get("/{id}", GetProjectHandler(deps))
	r.Patch("/{id}", UpdateProjectHandler(deps))
	r.Delete("/{id}", DeleteProjectHandler(deps))

	r.Route("/{projectId}/tasks", func(r chi.Router) {
		r.Post("/", tasks.CreateTaskHandler(deps))
		r.Get("/", tasks.ListProjectTasksHandler(deps))
	})
}

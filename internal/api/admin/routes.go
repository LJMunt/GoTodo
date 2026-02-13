package admin

import (
	"GoToDo/internal/api/config"
	"GoToDo/internal/api/languages"
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Get("/metrics", GetDatabaseMetricsHandler(deps.DB))

	r.Route("/config", func(r chi.Router) {
		config.AdminRoutes(r, deps)
	})

	r.Route("/lang", func(r chi.Router) {
		r.Get("/", languages.AdminListLanguagesHandler(deps.DB))
		r.Post("/", languages.AdminCreateLanguageHandler(deps.DB))
		r.Delete("/{code}", languages.AdminDeleteLanguageHandler(deps.DB))
	})

	r.Route("/users", func(r chi.Router) {
		r.Get("/", ListUsersHandler(deps.DB))
		r.Get("/{id}", GetUserHandler(deps.DB))
		r.Patch("/{id}", UpdateUserHandler(deps.DB))
		r.Delete("/{id}", DeleteUserHandler(deps.DB))

		r.Route("/{userId}/projects", func(r chi.Router) {
			r.Get("/", ListUserProjectsHandler(deps.DB))
			r.Get("/{projectId}", GetProjectHandler(deps.DB))
			r.Patch("/{projectId}", UpdateProjectHandler(deps.DB))
			r.Delete("/{projectId}", DeleteProjectHandler(deps.DB))
			r.Post("/{projectId}/restore", RestoreProjectHandler(deps.DB))
		})
		r.Route("/{userId}/tasks", func(r chi.Router) {
			r.Get("/", ListUserTasksHandler(deps.DB))
			r.Delete("/{taskId}", DeleteUserTaskHandler(deps.DB))
			r.Post("/{taskId}/restore", RestoreUserTaskHandler(deps.DB))
		})

		r.Route("/{userId}/projects/{projectId}/tasks", func(r chi.Router) {
			r.Get("/", ListProjectTasksHandler(deps.DB))
		})
		r.Route("/{userId}/tags", func(r chi.Router) {
			r.Get("/", ListUserTagsHandler(deps.DB))
			r.Delete("/{tagId}", DeleteUserTagHandler(deps.DB))
		})
	})
}

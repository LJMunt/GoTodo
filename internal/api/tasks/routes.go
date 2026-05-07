package tasks

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	// Global task endpoints
	r.Route("/tasks", func(r chi.Router) {
		r.Get("/{id}", GetTaskHandler(deps))
		r.Patch("/{id}", UpdateTaskHandler(deps))
		r.Delete("/{id}", DeleteTaskHandler(deps))

		r.Get("/{taskId}/tags", GetTaskTagsHandler(deps))
		r.Put("/{taskId}/tags", PutTaskTagsHandler(deps))

		// Occurrences (recurring tasks)
		r.Get("/{taskId}/occurrences", ListTaskOccurrencesHandler(deps))
		r.Patch("/{taskId}/occurrences/{occurrenceId}", UpdateTaskOccurrenceHandler(deps))
	})
}

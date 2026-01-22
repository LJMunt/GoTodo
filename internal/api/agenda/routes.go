package agenda

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	// GET /agenda?from=...&to=...
	r.Get("/", GetAgendaHandler(deps.DB))
}

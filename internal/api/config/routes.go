package config

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Get("/", GetConfigHandler(deps.DB))
}

func AdminRoutes(r chi.Router, deps app.Deps) {
	r.Get("/keys", ListConfigKeysHandler(deps.DB))
	r.Get("/translations", GetTranslationsHandler(deps.DB))
	r.Put("/translations", UpdateTranslationsHandler(deps.DB))

	// Backend (non-string) config values
	r.Get("/values", GetConfigValuesHandler(deps.DB))
	r.Put("/values", UpdateConfigValuesHandler(deps.DB))
}

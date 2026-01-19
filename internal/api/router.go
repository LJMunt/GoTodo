package api

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"GoToDo/internal/api/auth"
	"GoToDo/internal/api/users"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
)

func NewRouter(deps app.Deps) chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", HealthHandler())
		r.Get("/ready", ReadyHandler(deps.DB))

		r.Route("/auth", func(r chi.Router) {
			auth.Routes(r, deps)
		})

		r.Group(func(r chi.Router) {
			r.Use(authmw.RequireAuth(deps.DB))
			r.Route("/users", func(r chi.Router) {
				users.Routes(r, deps)
			})
		})
	})
	return r
}

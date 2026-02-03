package api

import (
	"GoToDo/internal/api/agenda"
	"GoToDo/internal/api/projects"
	"GoToDo/internal/api/tags"
	"GoToDo/internal/api/tasks"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"GoToDo/internal/api/admin"
	"GoToDo/internal/api/auth"
	"GoToDo/internal/api/users"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"github.com/go-chi/httprate"
	"github.com/unrolled/secure"
)

func NewRouter(deps app.Deps) chi.Router {
	r := chi.NewRouter()

	secureMiddleware := secure.New(secure.Options{
		FrameDeny:          true,
		ContentTypeNosniff: true,
		BrowserXssFilter:   true,
		// In production, you'd want these:
		// IsDevelopment: false,
		// STSSeconds: 31536000,
		// STSIncludeSubdomains: true,
	})

	// Global middleware
	r.Use(func(next http.Handler) http.Handler {
		return secureMiddleware.Handler(next)
	})
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(middleware.CleanPath)

	// Global rate limit: 100 requests per minute per IP
	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", HealthHandler())
		r.Get("/ready", ReadyHandler(deps.DB))
		r.Get("/version", VersionHandler())

		r.Route("/auth", func(r chi.Router) {
			// Brute force protection for auth routes: 10 requests per minute per IP
			r.Use(httprate.LimitByIP(10, 1*time.Minute))
			auth.Routes(r, deps)
		})

		// Admin routes
		r.Route("/admin", func(r chi.Router) {
			r.Use(authmw.RequireAuth(deps.DB))
			r.Use(authmw.RequireAdmin)
			admin.Routes(r, deps)
		})

		// Everything in here requires a valid user token
		r.Group(func(r chi.Router) {
			r.Use(authmw.RequireAuth(deps.DB))

			r.Route("/auth/password-change", func(r chi.Router) {
				// Brute force protection for password change: 10 requests per minute per IP
				r.Use(httprate.LimitByIP(10, 1*time.Minute))
				r.Post("/", auth.PasswordChangeHandler(deps.DB))
			})

			r.Route("/users", func(r chi.Router) {
				users.Routes(r, deps)
			})

			r.Route("/projects", func(r chi.Router) {
				projects.Routes(r, deps)
			})
			r.Route("/tags", func(r chi.Router) {
				tags.Routes(r, deps)
			})
			r.Route("/agenda", func(r chi.Router) {
				agenda.Routes(r, deps)
			})
			r.Group(func(r chi.Router) {
				tasks.Routes(r, deps)
			})
		})
	})

	return r
}

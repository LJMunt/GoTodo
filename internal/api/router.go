package api

import (
	"GoToDo/internal/api/agenda"
	"GoToDo/internal/api/orgs"
	"GoToDo/internal/api/projects"
	"GoToDo/internal/api/tags"
	"GoToDo/internal/api/tasks"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"GoToDo/internal/api/admin"
	"GoToDo/internal/api/auth"
	"GoToDo/internal/api/config"
	"GoToDo/internal/api/languages"
	"GoToDo/internal/api/users"
	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/logging"

	"github.com/go-chi/httprate"
	"github.com/unrolled/secure"
)

// Main Router

func NewRouter(deps app.Deps) chi.Router {
	r := chi.NewRouter()

	secureMiddleware := secure.New(secure.Options{
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		BrowserXssFilter:     true,
		IsDevelopment:        false,
		STSSeconds:           31536000,
		STSIncludeSubdomains: true,
	})

	// Global middleware
	r.Use(func(next http.Handler) http.Handler {
		return secureMiddleware.Handler(next)
	})
	r.Use(middleware.RequestID)
	r.Use(RealIPFromTrustedProxies(deps.Config.Server.TrustedProxies))
	r.Use(logging.RequestLogger(deps.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(middleware.CleanPath)
	r.Use(BodyLimitByPath(deps.Config.Server.MaxBodyBytes, deps.Config.Server.AdminMaxBodyBytes, "/api/v1/admin"))

	// Global rate limit: 200 requests per minute per IP
	r.Use(httprate.LimitByIP(200, 1*time.Minute))

	r.Use(authmw.ReadOnly(deps.DB))

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", HealthHandler())
		r.Get("/ready", ReadyHandler(deps.DB))
		r.Get("/version", VersionHandler())
		r.Get("/lang", languages.ListLanguagesHandler(deps.DB))
		r.Route("/config", func(r chi.Router) {
			r.Get("/status", config.GetConfigStatusHandler(deps.DB))
			config.Routes(r, deps)
		})

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

			r.Post("/auth/logout", auth.LogoutHandler(deps.DB))

			r.Route("/mfa", func(r chi.Router) {
				r.Post("/totp/start", auth.MfaTotpStartHandler(deps.DB))
				r.Post("/totp/confirm", auth.MfaTotpConfirmHandler(deps.DB))
				r.Post("/totp/disable", auth.MfaTotpDisableHandler(deps.DB))
			})

			r.Route("/auth/password-change", func(r chi.Router) {
				// Brute force protection for password change: 10 requests per minute per IP
				r.Use(httprate.LimitByIP(10, 1*time.Minute))
				r.Post("/", auth.PasswordChangeHandler(deps.DB))
			})

			r.Route("/users", func(r chi.Router) {
				users.Routes(r, deps)
			})
			r.Route("/orgs", func(r chi.Router) {
				orgs.Routes(r, deps)
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

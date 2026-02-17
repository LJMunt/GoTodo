package auth

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Post("/signup", SignupHandler(deps.DB))
	r.Post("/login", LoginHandler(deps.DB))
	r.Get("/verify-email", VerifyEmailHandler(deps.DB))
	r.Post("/verify-email", VerifyEmailHandler(deps.DB))
	r.Post("/verify-email/resend", ResendVerificationHandler(deps.DB))
}

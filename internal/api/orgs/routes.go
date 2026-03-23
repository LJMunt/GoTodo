package orgs

import (
	"GoToDo/internal/app"
	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Use(OrganizationsEnabled(deps.DB))

	r.Post("/", CreateOrganizationHandler(deps.DB))
	r.Get("/", ListOrganizationsHandler(deps.DB))

	r.Route("/{id}", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(RequireOrgMember(deps.DB))
			r.Get("/", GetOrganizationHandler(deps.DB))
			r.Get("/members", ListMembersHandler(deps.DB))
			r.Post("/leave", LeaveOrganizationHandler(deps.DB))
		})

		r.Group(func(r chi.Router) {
			r.Use(RequireOrgAdmin(deps.DB))
			r.Patch("/", UpdateOrganizationHandler(deps.DB))
			r.Delete("/", DeleteOrganizationHandler(deps.DB))

			r.Post("/members", AddMemberHandler(deps.DB))
			r.Delete("/members/{userId}", RemoveMemberHandler(deps.DB))
		})
	})
}

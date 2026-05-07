package orgs

import (
	"GoToDo/internal/app"
	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Use(OrganizationsEnabled(deps))

	r.Post("/", CreateOrganizationHandler(deps))
	r.Get("/", ListOrganizationsHandler(deps))

	r.Route("/{id}", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(RequireOrgMember(deps))
			r.Get("/", GetOrganizationHandler(deps))
			r.Get("/members", ListMembersHandler(deps))
			r.Post("/leave", LeaveOrganizationHandler(deps))
		})

		r.Group(func(r chi.Router) {
			r.Use(RequireOrgAdmin(deps))
			r.Patch("/", UpdateOrganizationHandler(deps))
			r.Delete("/", DeleteOrganizationHandler(deps))

			r.Post("/members", AddMemberHandler(deps))
			r.Delete("/members/{userId}", RemoveMemberHandler(deps))
		})
	})
}

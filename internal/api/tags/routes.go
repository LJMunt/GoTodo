package tags

import (
	"GoToDo/internal/app"
	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Get("/", ListTagsHandler(deps.DB))
	r.Post("/", CreateTagHandler(deps.DB))
	r.Patch("/{tagId}", RenameTagHandler(deps.DB))
	r.Delete("/{tagId}", DeleteTagHandler(deps.DB))
}

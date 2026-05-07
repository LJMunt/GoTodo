package tags

import (
	"GoToDo/internal/app"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, deps app.Deps) {
	r.Get("/", ListTagsHandler(deps))
	r.Post("/", CreateTagHandler(deps))
	r.Patch("/{tagId}", RenameTagHandler(deps))
	r.Delete("/{tagId}", DeleteTagHandler(deps))
}

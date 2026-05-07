package app

import (
	"GoToDo/internal/repository"
	"GoToDo/internal/service"
	"github.com/rs/zerolog"
)

type Deps struct {
	DB             repository.DBTX
	Logger         zerolog.Logger
	Config         Config
	TaskService    service.TaskService
	ProjectService service.ProjectService
	TagService     service.TagService
	UserService    service.UserService
	OrgService     service.OrgService
}

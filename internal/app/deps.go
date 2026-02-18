package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type Deps struct {
	DB     *pgxpool.Pool
	Logger zerolog.Logger
	Config Config
}

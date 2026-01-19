package app

import "github.com/jackc/pgx/v5/pgxpool"

type Deps struct {
	DB *pgxpool.Pool
}

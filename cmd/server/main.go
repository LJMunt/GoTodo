package main

import (
	"GoToDo/internal/api"
	"GoToDo/internal/app"
	"GoToDo/internal/db"
	"GoToDo/internal/logging"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := logging.Init()
	log.Logger = logger

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Fatal().Msg("DATABASE_URL environment variable not set")
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "internal/db/migrations"
	}

	logger.Info().Str("path", migrationsPath).Msg("Running migrations...")
	if err := db.Migrate(dsn, migrationsPath); err != nil {
		logger.Fatal().Err(err).Msg("migrations failed")
	}
	logger.Info().Msg("Migrations completed successfully")

	//DB Connection
	pool, err := db.Connect(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("db connect failed")
	}
	defer pool.Close()

	// Start level refresher
	logging.StartLevelRefresher(ctx, logger, &logging.DBLevelSource{Pool: pool}, 5*time.Second)

	// Start configuration sanity checker (checks every 1 minute)
	logging.StartConfigWatcher(ctx, logger, pool, 1*time.Minute)

	r := api.NewRouter(app.Deps{DB: pool, Logger: logger})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	logger.Info().Str("addr", addr).Msg("listening")
	logger.Fatal().Err(http.ListenAndServe(addr, r)).Msg("server stopped")
}

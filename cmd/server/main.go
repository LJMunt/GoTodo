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

	"github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := logging.Init()
	log.Logger = logger

	cfg, err := app.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("config load failed")
	}

	logger.Info().Str("path", cfg.MigrationsPath).Msg("Running migrations...")
	if err := db.Migrate(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		logger.Fatal().Err(err).Msg("migrations failed")
	}
	logger.Info().Msg("Migrations completed successfully")

	//DB Connection
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("db connect failed")
	}
	defer pool.Close()

	// Start level refresher
	logging.StartLevelRefresher(ctx, logger, &logging.DBLevelSource{Pool: pool}, cfg.Logging.LevelRefreshInterval)

	// Start configuration sanity checker (checks every 1 minute)
	logging.StartConfigWatcher(ctx, logger, pool, cfg.Logging.ConfigWatchInterval)

	r := api.NewRouter(app.Deps{DB: pool, Logger: logger, Config: cfg})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	errCh := make(chan error, 1)
	logger.Info().Str("addr", addr).Msg("listening")
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("graceful shutdown failed")
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server stopped")
		}
	}
}

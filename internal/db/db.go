package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}

	// Defaults
	cfg.MaxConns = 10
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 10 * time.Minute
	cfg.MaxConnIdleTime = 10 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	const (
		maxAttempts = 30
		delay       = 500 * time.Millisecond
	)

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = pool.Ping(pingCtx)
			cancel()

			if err == nil {
				return pool, nil
			}

			pool.Close()
			lastErr = fmt.Errorf("ping failed: %w", err)
		} else {
			lastErr = fmt.Errorf("pgxpool.NewWithConfig: %w", err)
		}

		// Stop retrying if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Wait before retrying (unless this was the last attempt)
		if attempt < maxAttempts {
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf(
		"database not ready after %d attempts (~%s): %w",
		maxAttempts,
		time.Duration(maxAttempts)*delay,
		lastErr,
	)
}

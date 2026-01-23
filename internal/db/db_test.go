package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	origDSN := os.Getenv("DATABASE_URL")
	defer os.Setenv("DATABASE_URL", origDSN)

	t.Run("Missing DATABASE_URL", func(t *testing.T) {
		os.Unsetenv("DATABASE_URL")
		pool, err := Connect(context.Background())
		if err == nil {
			pool.Close()
			t.Error("expected error when DATABASE_URL is missing")
		}
	})

	t.Run("Invalid DATABASE_URL format", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "invalid-dsn")
		pool, err := Connect(context.Background())
		if err == nil {
			pool.Close()
			t.Error("expected error for invalid DATABASE_URL format")
		}
	})

	t.Run("Valid Connection", func(t *testing.T) {
		if origDSN == "" {
			t.Skip("DATABASE_URL not set, skipping valid connection test")
		}
		os.Setenv("DATABASE_URL", origDSN)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := Connect(ctx)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			t.Errorf("failed to ping: %v", err)
		}
	})

	t.Run("Context Cancelled", func(t *testing.T) {
		if origDSN == "" {
			t.Skip("DATABASE_URL not set, skipping context cancelled test")
		}
		os.Setenv("DATABASE_URL", origDSN)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		pool, err := Connect(ctx)
		if err == nil {
			pool.Close()
			t.Error("expected error for cancelled context")
		}
	})
}

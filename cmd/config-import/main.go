package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"GoToDo/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type configKey struct {
	Key         string           `json:"key"`
	Description *string          `json:"description,omitempty"`
	DataType    string           `json:"data_type"`
	IsPublic    bool             `json:"is_public"`
	IsSecret    bool             `json:"is_secret"`
	ValueJSON   *json.RawMessage `json:"value_json,omitempty"`
}

type importPayload struct {
	Keys []configKey `json:"keys"`
}

func main() {
	inPath := flag.String("in", "", "input file path")
	flag.Parse()

	if strings.TrimSpace(*inPath) == "" {
		log.Fatal("missing --in file path")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_ = godotenv.Load()

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	payload, err := readPayload(*inPath)
	if err != nil {
		log.Fatalf("failed to read payload: %v", err)
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := applyImport(ctx, pool, payload); err != nil {
		log.Fatalf("import failed: %v", err)
	}
}

func readPayload(path string) (importPayload, error) {
	var payload importPayload
	f, err := os.Open(path)
	if err != nil {
		return payload, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func applyImport(ctx context.Context, pool *pgxpool.Pool, payload importPayload) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, key := range payload.Keys {
		if key.IsPublic {
			continue
		}

		var raw any
		if key.ValueJSON != nil {
			raw = string(*key.ValueJSON)
		} else {
			raw = nil
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO config_keys (key, description, data_type, is_public, is_secret, value_json, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, now())
			ON CONFLICT (key) DO UPDATE SET
				description = EXCLUDED.description,
				data_type = EXCLUDED.data_type,
				is_public = EXCLUDED.is_public,
				is_secret = EXCLUDED.is_secret,
				value_json = EXCLUDED.value_json,
				updated_at = now()
		`, key.Key, key.Description, key.DataType, key.IsPublic, key.IsSecret, raw); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

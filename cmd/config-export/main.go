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
	"time"

	"GoToDo/internal/db"

	"github.com/jackc/pgx/v5"
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

type exportPayload struct {
	ExportedAt    time.Time   `json:"exported_at"`
	Keys          []configKey `json:"keys"`
	SchemaVersion int         `json:"schema_version"`
}

func main() {
	outPath := flag.String("out", "config-export.json", "output file path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_ = godotenv.Load()

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	keys, err := loadKeys(ctx, pool)
	if err != nil {
		log.Fatalf("failed to load config keys: %v", err)
	}
	payload := exportPayload{
		ExportedAt:    time.Now().UTC(),
		Keys:          keys,
		SchemaVersion: 1,
	}

	f, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		log.Fatalf("failed to write export: %v", err)
	}
}

func loadKeys(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}) ([]configKey, error) {
	rows, err := q.Query(ctx, `
		SELECT key, description, data_type, is_public, is_secret, value_json
		FROM config_keys
		WHERE is_public = false
		ORDER BY key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []configKey
	for rows.Next() {
		var k configKey
		var raw []byte
		if err := rows.Scan(&k.Key, &k.Description, &k.DataType, &k.IsPublic, &k.IsSecret, &raw); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			msg := json.RawMessage(raw)
			k.ValueJSON = &msg
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

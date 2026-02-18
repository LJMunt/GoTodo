package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

type exportPayload struct {
	ExportedAt    time.Time        `json:"exported_at"`
	Users         []map[string]any `json:"users"`
	Projects      []map[string]any `json:"projects"`
	Tasks         []map[string]any `json:"tasks"`
	Tags          []map[string]any `json:"tags"`
	TaskTags      []map[string]any `json:"task_tags"`
	Occurrences   []map[string]any `json:"task_occurrences"`
	SchemaVersion int              `json:"schema_version"`
}

func main() {
	outPath := flag.String("out", "user-export.json", "output file path")
	tokenize := flag.Bool("tokenize", false, "anonymize user identifiers and emails")
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

	users, err := queryTable(ctx, pool, "SELECT * FROM users ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to export users: %v", err)
	}
	projects, err := queryTable(ctx, pool, "SELECT * FROM projects ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to export projects: %v", err)
	}
	tasks, err := queryTable(ctx, pool, "SELECT * FROM tasks ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to export tasks: %v", err)
	}
	tags, err := queryTable(ctx, pool, "SELECT * FROM tags ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to export tags: %v", err)
	}
	taskTags, err := queryTable(ctx, pool, "SELECT * FROM task_tags ORDER BY task_id ASC, tag_id ASC")
	if err != nil {
		log.Fatalf("failed to export task_tags: %v", err)
	}
	occurrences, err := queryTable(ctx, pool, "SELECT * FROM task_occurrences ORDER BY id ASC")
	if err != nil {
		log.Fatalf("failed to export task_occurrences: %v", err)
	}

	if *tokenize {
		tokenizeUsers(users)
	}

	payload := exportPayload{
		ExportedAt:    time.Now().UTC(),
		Users:         users,
		Projects:      projects,
		Tasks:         tasks,
		Tags:          tags,
		TaskTags:      taskTags,
		Occurrences:   occurrences,
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

func queryTable(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, sql string) ([]map[string]any, error) {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	out := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(values))
		for i, fd := range fields {
			row[string(fd.Name)] = values[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func tokenizeUsers(rows []map[string]any) {
	for _, row := range rows {
		id, ok := row["id"]
		if !ok {
			continue
		}
		token := fmt.Sprintf("user-%v", id)

		if _, ok := row["email"]; ok {
			row["email"] = fmt.Sprintf("%s@example.invalid", token)
		}
		if _, ok := row["public_id"]; ok {
			row["public_id"] = token
		}
		if _, ok := row["password_hash"]; ok {
			row["password_hash"] = ""
		}
	}
}

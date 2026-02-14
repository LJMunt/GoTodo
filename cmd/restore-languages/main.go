package main

import (
	"GoToDo/internal/db"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load .env file if it exists
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	// Connect to database
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Locate SQL file
	sqlPath := os.Getenv("RESTORE_SQL_PATH")
	if sqlPath == "" {
		// Try to find the project root by looking for go.mod.
		root := "."
		foundRoot := false
		for range 5 { // Look up to 5 levels up
			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				foundRoot = true
				break
			}
			root = filepath.Join("..", root)
		}

		candidates := []string{}
		if foundRoot {
			candidates = append(candidates, filepath.Join(root, "internal", "db", "restore_languages.sql"))
		}

		// Try next to the binary (useful for container images).
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			candidates = append(candidates, filepath.Join(exeDir, "internal", "db", "restore_languages.sql"))
		}

		// Common container path.
		candidates = append(candidates, "/app/internal/db/restore_languages.sql")

		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				sqlPath = candidate
				break
			}
		}
	}

	content, err := os.ReadFile(sqlPath)
	if err != nil {
		log.Fatalf("failed to read SQL file at %s: %v", sqlPath, err)
	}

	log.Printf("Executing restore script from %s...", sqlPath)

	_, err = pool.Exec(ctx, string(content))
	if err != nil {
		log.Fatalf("failed to execute SQL: %v", err)
	}

	fmt.Println("Successfully restored default languages and translations.")
}

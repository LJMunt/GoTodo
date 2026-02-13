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
		// Try to find the project root by looking for go.mod
		root := "."
		for range 5 { // Look up to 5 levels up
			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				break
			}
			root = filepath.Join("..", root)
		}
		sqlPath = filepath.Join(root, "internal", "db", "restore_languages.sql")
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

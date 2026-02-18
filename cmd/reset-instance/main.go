package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"GoToDo/internal/db"

	"github.com/joho/godotenv"
)

const confirmPhrase = "RESET INSTANCE"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) != 1 {
		log.Fatalf("usage: %s", os.Args[0])
	}

	// Load .env file if it exists
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "internal/db/migrations"
	}

	if err := confirm(); err != nil {
		log.Fatal(err)
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO public;
		GRANT ALL ON SCHEMA public TO CURRENT_USER;
	`); err != nil {
		log.Fatalf("failed to reset schema: %v", err)
	}

	if err := db.Migrate(dsn, migrationsPath); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	fmt.Println("Instance reset complete.")
}

func confirm() error {
	if !isTerminal(os.Stdin) {
		return fmt.Errorf("stdin is not a terminal; refusing to run without a TTY")
	}

	fmt.Println("DANGER: This will permanently delete ALL data in the database and recreate the schema.")
	fmt.Printf("Type %q to continue: ", confirmPhrase)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if strings.TrimSpace(line) != confirmPhrase {
		return fmt.Errorf("confirmation phrase did not match; aborting")
	}

	code, err := confirmationCode()
	if err != nil {
		return fmt.Errorf("failed to generate confirmation code: %w", err)
	}
	fmt.Printf("Type %s to confirm reset: ", code)
	line, err = reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if strings.TrimSpace(line) != code {
		return fmt.Errorf("confirmation code did not match; aborting")
	}
	return nil
}

func confirmationCode() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

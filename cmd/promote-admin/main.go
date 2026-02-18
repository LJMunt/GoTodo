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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <user-email>", os.Args[0])
	}

	email := strings.TrimSpace(strings.ToLower(os.Args[1]))
	if email == "" {
		log.Fatal("email must not be empty")
	}

	// Load .env file if it exists
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	if !confirm() {
		log.Fatal("confirmation failed; aborting")
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	tag, err := pool.Exec(ctx, "UPDATE users SET is_admin = true, updated_at = now() WHERE email = $1", email)
	if err != nil {
		log.Fatalf("failed to promote user: %v", err)
	}
	if tag.RowsAffected() == 0 {
		log.Fatalf("no user found with email %s", email)
	}

	fmt.Printf("User %s promoted to admin.\n", email)
}

func confirm() bool {
	code, err := confirmationCode()
	if err != nil {
		return false
	}

	fmt.Printf("Type %s to confirm admin promotion: ", code)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(line) == code
}

func confirmationCode() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

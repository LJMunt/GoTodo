package main

import (
	"GoToDo/internal/api"
	"GoToDo/internal/app"
	"GoToDo/internal/db"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "internal/db/migrations"
	}

	log.Printf("Running migrations from %s...", migrationsPath)
	if err := db.Migrate(dsn, migrationsPath); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}
	log.Println("Migrations completed successfully")

	//DB Connection
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer pool.Close()

	r := api.NewRouter(app.Deps{DB: pool})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

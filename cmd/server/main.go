package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"bahago/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {

	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Expected new pgxpool created: %v", err)
	}
	defer db.Close()

	srv := server.New(db)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: srv,
		// Time to read the full request including body.
		ReadTimeout: 5 * time.Second,
		// WriteTimeout is 0 (no timeout) because SSE endpoints hold open
		// long-lived connections and would be cut off by a finite timeout.
		WriteTimeout: 0,
		// Time to wait for the next request on a keep-alive connection.
		IdleTimeout: 120 * time.Second,
	}

	fmt.Println("Server running at http://localhost:8080")
	log.Fatal(httpServer.ListenAndServe())
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"bahago/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {

	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Expected new pgxpool created: %v", err)
	}
	defer db.Close()

	app := server.New(db)

	// start server
	fmt.Println("Server running at http://localhost:8080")
	http.ListenAndServe(":8080", app)
}

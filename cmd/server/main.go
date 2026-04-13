package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"bahago/internal/email"
	"bahago/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// --- signal ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// --- config ---
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	resendAPIKey := os.Getenv("RESEND_API_KEY")
	if resendAPIKey == "" {
		log.Fatal("RESEND_API_KEY environment variable is not set")
	}

	emailFrom := os.Getenv("EMAIL_FROM")
	if emailFrom == "" {
		log.Fatal("EMAIL_FROM environment variable is not set")
	}

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		log.Fatal("APP_URL environment variable is not set")
	}

	// --- database ---
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer pool.Close()

	// --- application wiring ---
	sender := email.NewSender(resendAPIKey, emailFrom)
	srv := server.New(pool, sender, appURL)

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

	// --- start ---
	fmt.Println("Server running at http://localhost:8080")

	go srv.StartGameTicker(ctx)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// --- shutdown ---
	<-ctx.Done()
	stop()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

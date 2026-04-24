// Package testhelper provides shared utilities for database integration tests.
package testhelper

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"bahago/internal/database/db"
	"bahago/internal/router"
)

// migrationsDir returns the absolute path to the migrations directory,
// resolved relative to the location of this source file so it works
// regardless of the working directory tests are run from.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "database", "migrations")
}

// EnsureSchema connects to the test database at dsn, creating it if necessary,
// and runs all goose migrations up. It is safe to call multiple times (goose Up
// is idempotent). Returns an error rather than panicking so it can be called
// from TestMain as well as from individual tests.
func EnsureSchema(dsn string) error {
	ctx := context.Background()

	// Ensure the test database exists before attempting migrations.
	rootDSN := strings.Replace(dsn, "/testdb", "/postgres", 1)
	if sqlRoot, err := sql.Open("pgx", rootDSN); err == nil {
		sqlRoot.ExecContext(ctx, "CREATE DATABASE testdb") //nolint:errcheck // ignore "already exists"
		sqlRoot.Close()
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose.SetDialect: %w", err)
	}
	goose.SetLogger(goose.NopLogger())

	if err := goose.UpContext(ctx, sqlDB, migrationsDir()); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// SetupDB connects to the test database (TEST_DATABASE_URL), creates it if
// necessary, runs all goose migrations up, and returns a connection pool.
// The pool is closed via t.Cleanup when the test finishes.
// Tests are skipped if TEST_DATABASE_URL is not set.
func SetupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	testDSN := os.Getenv("TEST_DATABASE_URL")
	if testDSN == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}

	if err := EnsureSchema(testDSN); err != nil {
		t.Fatalf("testhelper: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		t.Fatalf("testhelper: pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// WithRollback begins a transaction on the pool and returns a db.Querier backed
// by that transaction. The transaction is rolled back in t.Cleanup, so every
// test using this helper starts with a clean slate with no teardown SQL needed.
func WithRollback(t *testing.T, pool *pgxpool.Pool) db.Querier {
	t.Helper()

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("testhelper: begin tx: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(ctx)
	})

	return db.New(tx)
}

// CaptureRouter is a fake router.Router that stores registered handlers in a
// map keyed by "METHOD /path". Pass it to a feature's RegisterRoutes function,
// then extract individual handlers for direct testing without a real HTTP server.
type CaptureRouter struct {
	Handlers map[string]http.HandlerFunc
}

func (cr *CaptureRouter) Handle(pattern string, h http.Handler) {
	cr.Handlers[pattern] = h.ServeHTTP
}

func (cr *CaptureRouter) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	cr.Handlers[pattern] = h
}

var _ router.Router = (*CaptureRouter)(nil)

// NewCaptureRouter returns an initialised CaptureRouter ready for use.
func NewCaptureRouter() *CaptureRouter {
	return &CaptureRouter{Handlers: make(map[string]http.HandlerFunc)}
}

// AssertContains fails the test if body does not contain want.
func AssertContains(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Errorf("response body = %q; want to contain %q", body, want)
	}
}

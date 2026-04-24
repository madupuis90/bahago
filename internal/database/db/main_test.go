package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"bahago/internal/testhelper"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println("TEST_DATABASE_URL not set; skipping DB integration tests")
		return 0
	}

	if err := testhelper.EnsureSchema(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "db_test: %v\n", err)
		return 1
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db_test: pgxpool.New: %v\n", err)
		return 1
	}
	defer pool.Close()
	testPool = pool

	return m.Run()
}

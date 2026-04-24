---
description: "Use when writing or reviewing test files in this project"
applyTo: "**/*_test.go"
---

# Testing Conventions

## Test Types

There are two distinct test flavours in this project:

### 1. DB Integration Tests (`internal/database/db/`)

Tests in `internal/database/db/` verify sqlc queries against a real PostgreSQL database. They require `TEST_DATABASE_URL` to be set and are skipped when it is not.

**Setup pattern** — each package with DB tests uses `TestMain` + a shared pool:

```go
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
```

**Each test gets an isolated transaction** via `testhelper.WithRollback`. No teardown SQL is needed — the transaction is always rolled back:

```go
func TestCreateUser_Success(t *testing.T) {
    q := testhelper.WithRollback(t, testPool)
    // ... test using q
}
```

**Seed helpers** for complex fixtures live at the top of the test file and are named `seed<Thing>Fixture`. They call `t.Helper()` and `t.Fatalf` (not `t.Errorf`) on setup failures:

```go
func seedCampaignFixture(t *testing.T, q db.Querier, attackerUnits int) (attacker, target db.Kingdom) {
    t.Helper()
    // ...
}
```

### 2. Handler Unit Tests (`internal/handlers/<feature>/`)

Tests in handler packages verify HTTP handler behaviour without a real database. They use a **stub querier** pattern:

```go
// stubQuerier embeds a nil db.Querier. Any method not explicitly overridden
// panics via a nil pointer dereference, making unexpected DB calls immediately
// visible. Override only the methods a specific test expects to be called.
type stubQuerier struct {
    db.Querier
    onGetKingdomByName func(ctx context.Context, name string) (db.Kingdom, error)
}

func (s *stubQuerier) GetKingdomByName(ctx context.Context, name string) (db.Kingdom, error) {
    if s.onGetKingdomByName != nil {
        return s.onGetKingdomByName(ctx, name)
    }
    panic("stubQuerier: unexpected call to GetKingdomByName")
}
```

**Extract handlers via `CaptureRouter`** — don't start a real HTTP server:

```go
func sendHandler(q db.Querier) http.HandlerFunc {
    cr := testhelper.NewCaptureRouter()
    army.RegisterRoutes(cr, q, nil, nil)
    return cr.Handlers["POST "+routes.KingdomArmySendPath]
}
```

**Inject context values** to simulate middleware:

```go
func sendReq(body string, kingdom *db.Kingdom) *http.Request {
    r := httptest.NewRequest("POST", routes.KingdomArmySendPath, strings.NewReader(body))
    return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}
```

## General Rules

- Test function names follow `TestFunctionName_Scenario` format
- Use `t.Fatalf` for setup failures (stops test immediately); `t.Errorf` for assertion failures (continues test)
- Call `t.Helper()` in every test helper function
- Use `t.Cleanup()` for resource teardown (preferred over `defer` in tests)
- Write table-driven tests when you have multiple input/output scenarios for the same behaviour:
  ```go
  tests := []struct {
      name  string
      input string
      want  int
  }{
      {"empty", "", 0},
      {"single", "a", 1},
  }
  for _, tc := range tests {
      t.Run(tc.name, func(t *testing.T) { ... })
  }
  ```
- Prefer `testhelper.AssertContains(t, body, want)` for HTTP response body checks
- Check `pgx.ErrNoRows` explicitly when testing not-found cases — don't just check `err != nil`

## Game Logic Tests (`internal/game/`)

Pure Go tests — no database, no stubs. Test with direct function calls and table-driven cases. These are the fastest and most granular tests in the codebase.

## What NOT to Test

- Do not test sqlc-generated code itself — it is generated and tested upstream
- Do not test middleware in handler unit tests — middleware guarantees are asserted by integration tests
- Do not re-test framework behaviour (e.g., that `http.Error` sets the right status code)

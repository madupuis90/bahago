# GitHub Copilot Instructions for Bahago

## Communication & Approach
- Never make up information - if uncertain, research or ask for clarification
- Always use project instruction if present or research best practices when no instruction provided
- Always give answers based on the specific version of tools/frameworks being used in the project
- Research current recommendations before suggesting patterns or approaches
- Provide context for decisions
- Provide reasoning behind suggestions to help build understanding
- Use file reading tools to read files directly — do not use terminal commands like `cat` or `echo` to read file contents. If a file cannot be accessed, ask the user to open it in the editor

## Project Overview
This is a Go web application using pgx/v5 for PostgreSQL 18, sqlc for type-safe queries, goose for migrations, gomponents for HTML templating and datastar for interactivity.

The project runs in a dev container and uses Task (Taskfile.yml) for all development commands. Never suggest running tools directly (like `goose`, `sqlc`, `air`, etc.) - always use the corresponding `task` command instead. When suggesting container commands, use Podman (not Docker) unless Docker is explicitly requested. Podman is rootless by default and uses a daemonless architecture, making it the preferred container runtime for Bazzite.

## PostgreSQL 18 Best Practices
- Use PostgreSQL 18 features and syntax
- Prefer `GENERATED ALWAYS AS IDENTITY` over `SERIAL` for primary keys
- Use `TEXT` type instead of `VARCHAR` unless you need length constraints
- Use `TIMESTAMPTZ` (timestamp with timezone) for datetime fields
- Add indexes on foreign keys and frequently queried columns
- Use `JSONB` (not `JSON`) for JSON data - it's faster and indexable
- Use `pg_trgm` extension for fuzzy text search when needed
- Use `uuid-ossp` or `gen_random_uuid()` for UUID generation

## Go Best Practices

See `.github/instructions/go.instructions.md` — auto-loaded for all `.go` files.

## UI Best Practices

See `.github/instructions/ui.instructions.md`. **Always read this file before making any change that involves gomponents, element IDs, signal names, SSE responses, or HTML templates.** The rules there (no string literals for IDs, paths, or signal names) must be applied proactively, not just when a violation is pointed out.

## SQL & Migration Best Practices

See `.github/instructions/sql.instructions.md`. **Always read this file before writing or reviewing SQL query files or migrations.** Covers sqlc naming conventions, bulk operation patterns, migration rules, and PostgreSQL type defaults.

## Testing Best Practices

See `.github/instructions/testing.instructions.md`. **Always read this file before writing or reviewing test files.** Covers the DB integration test pattern (`WithRollback`, `TestMain`), the stub querier pattern for handler tests, and `CaptureRouter` usage.

## UI Test Plans

See `.github/instructions/ui-test-plans.instructions.md`. **Always read this file when asked to run a UI test plan.** Test plan files live in `test/ui/<feature>.md`. When asked to run a plan, start `task dev` if needed, log in with the precondition account (password: `12345678`), then execute steps and report PASS/FAIL per step.

When making changes to a feature that has a test plan file, run all plans in that file after the changes are complete. If any plan fails, stop and report the failure to the user — do not silently ignore it, do not auto-fix the plan, and do not proceed with further changes until the user decides how to handle it.

## Database Practices

### Connection Management
- Always use `pgxpool.Pool` for connection pooling (never single connections in production)
- Pass `*pgxpool.Pool` or the sqlc-generated interface to handlers/services
- Set appropriate pool size based on application needs
- Always use context with timeouts for database operations

### Queries with sqlc
- Write all SQL in `internal/database/queries/*.sql` files
- One SQL file per feature, named after the feature (e.g. `auth.sql`, `chat.sql`)
- Use sqlc naming conventions: `-- name: GetUser :one`, `-- name: ListUsers :many`
- Keep queries simple and readable
- Use parameterized queries (never string concatenation)
- Generated code goes in `internal/database/db/` - never edit it manually

### Tick query performance
- Avoid N queries inside the tick loop — each per-row query multiplies with player count
- Prefer bulk queries: collect IDs/values in Go, then issue a single `WHERE id = ANY($1)` query
- Use CTEs with `RETURNING` to decrement and collect completed rows in one round trip (see `DecrementAndList*AtZero` pattern)
- When a loop update is unavoidable, document why a bulk alternative wasn't possible

### Migrations with goose
- All migrations in `internal/database/migrations/`
- Create migrations using task commands (see Taskfile.yml for available commands)
- Always write both Up and Down migrations
- Name migrations descriptively: `YYYYMMDDHHMMSS_description.sql`
- Keep migrations small and focused
- Run migrations with `task db:up` and `task db:down`
- Test migrations both up and down before committing
- Never modify existing migrations that have been deployed

### CHECK constraint style
- **Inline** for simple single-column numeric bounds: `count INT NOT NULL CHECK (count >= 0)`
- **Named constraint** for enumeration checks and multi-column checks — these are likely to be altered as the domain grows:
  ```sql
  CONSTRAINT missions_action_valid CHECK (action IN ('attack', 'defend')),
  CONSTRAINT sessions_expiry_future CHECK (expires_at > created_at)
  ```
- Never mix both styles on the same column

## Project Structure

### Directory Organization

All code lives under `internal/` — nothing is intended to be imported externally.

- `cmd/server/main.go` — entry point; minimal wiring only (routes, middleware, server start)
- `internal/contextkeys/` — shared context key constants (avoids import cycles)
- `internal/routes/` — shared route path constants (avoids import cycle between `internal/ui` and feature packages)
- `internal/database/db/` — sqlc-generated code (never edit manually)
- `internal/database/migrations/` — goose migration files
- `internal/database/queries/` — SQL query files for sqlc
- `internal/email/` — email sending
- `internal/ui/` — shared gomponents: layout shell (`HomeLayout`, `KingdomLayout`), nav components, and shared UI helpers (alerts, etc.). Dot-imported.
- `internal/middleware/` — HTTP middleware (auth, and future: logging, CSRF, etc.)
- `internal/handlers/<feature>/` — one package per feature; contains HTTP handlers, page functions, and components specific to that feature
- `internal/router/` — router interface (avoids circular imports when injecting middleware)
- `internal/server/` — application wiring: routes, middleware registration, static file serving
- `web/static/` — static assets (CSS, JS)

### Feature Package Structure

Each feature in `internal/handlers/<feature>/` follows this pattern:
- One file (typically `<feature>.go`) exports route constants **in `internal/routes/`**, registers routes, and defines the `handler` struct
- Handler methods return `http.HandlerFunc` and are kept thin — they read input, call queries, and render responses
- Content functions are pure functions returning `Node` — they take only domain data as parameters, never user, path, or request. Co-located with their handlers.
- Reusable components stay in the feature package until a second package needs them, then move to `internal/ui/`

### Handlers

Handlers return `http.HandlerFunc` closures, which allows pre-computation outside the request loop (e.g., generating sentinel hashes, preparing queries):

```go
func (h *handler) handleKingdomPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
        KingdomLayout(r, "Kingdom", kingdomContent(kingdom)).Render(w)
    }
}
```

- Full-page responses call `HomeLayout(r, title, content...)` or `KingdomLayout(r, title, content...)` — these read user/kingdom/path from the request context directly; no explicit `Content-Type` needed (gomponents sets it)
- `internal/ui` is dot-imported in all handler packages so `HomeLayout` and `KingdomLayout` are used without a prefix
- SSE handlers use `datastar.NewSSE` — see `ui.instructions.md` for ordering rules
- Log errors server-side; return generic messages to clients
- Use `http.Error()` only for non-SSE error responses (before `NewSSE` is called)
- Do not re-check middleware guarantees inside handlers. If a route is behind `RequireAuth`, trust that a user is in context — no `if user == nil` guard needed. Same for `LoadKingdom` on kingdom routes. Defensive checks at internal wiring boundaries add noise and imply to readers that the scenario is actually possible.

## Configuration

### Environment Variables
- Non-sensitive config (port, DSN for local dev) lives in `docker-compose.yml`
- Sensitive or environment-specific values (API keys, tokens) go in `.env` — see `.env.example` for all required keys
- Never commit `.env`; always keep `.env.example` up to date when adding new required variables
- `main.go` reads required variables at startup and calls `log.Fatal` if any are missing — fail fast rather than silently misbehave at runtime

### Task Runner
- Use Taskfile.yaml for ALL development tasks (required in dev container)
- Run `task dev` for development with live reload
- Run `task gen` to regenerate sqlc code after query changes
- Run `task db:up` and `task db:down` for migrations
- Never suggest running tools directly (goose, sqlc, air) - always use task commands

## Testing

### General Testing
- Write tests for business logic
- Use table-driven tests for multiple scenarios
- Name test functions clearly: `TestFunctionName_Scenario`
- Use `t.Helper()` in test helper functions
- Mock database interactions using interfaces

### Test Organization
- Keep tests in the same package with `_test.go` suffix
- Use `testdata/` directory for test fixtures
- Clean up resources in tests (use `t.Cleanup()`)

## Security

### General Security
- Never log sensitive data (passwords, tokens, personal info)
- Use prepared statements (sqlc does this automatically)
- Validate and sanitize user input
- Use HTTPS in production
- Keep dependencies updated

### Database Security
- Use connection strings from environment variables
- Never commit credentials
- Use least privilege database users
- Enable SSL/TLS for database connections in production

## Code Review Workflow

See `.github/instructions/review.instructions.md` for the full checklist. **Always read this file when asked to do a code review.**

The workflow is:
1. Run `git log --oneline` to identify the commits belonging to the feature being reviewed
2. Run `git diff <first-commit>^..HEAD` to pull the full diff of those changes
3. Read all changed files in full before forming any opinion
4. Present a structured review: what looks good, what needs to change (with specific file/line references), and any questions
5. Work through each issue with the user one at a time — do not batch-fix everything at once

Note: the project currently commits directly to main (solo dev, one feature at a time). If branches are introduced in the future, use `git diff main...HEAD` instead.

## Instruction Conflicts

When a user's request contradicts something in the project instructions, **always point it out explicitly** before proceeding. State which instruction applies, what the user asked, and offer the choice: update the instruction, make a one-time exception, or reconsider the approach.

More broadly: if something in the conversation seems worth capturing as a rule or convention — a pattern that keeps coming up, a decision that took effort to reach, or a preference that would otherwise be lost — raise it proactively. The user prefers keeping instructions accurate over silently diverging from them.

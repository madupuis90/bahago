# Bahago

## Domain Docs
- `CONTEXT.md` at the repo root defines the ubiquitous language for this codebase — canonical terms, what to avoid, and how domain experts talk about the game. Read it when naming things or discussing game concepts.
- `docs/adr/` (created lazily) contains architectural decision records for hard-to-reverse, non-obvious decisions.

## Communication & Approach
- Never make up information — if uncertain, research or ask for clarification
- Always use project instructions when present, or research best practices when none are provided
- Always give answers based on the specific version of tools/frameworks used in this project
- Provide context and reasoning behind suggestions
- Use the Read tool to read files directly — do not use shell commands like `cat` or `echo`

## Session Management

Long conversations multiply token cost: each turn re-reads the full accumulated context. Break large tasks into sessions using `/handoff` and `/pickup`.

**Proactively suggest `/handoff` when:**
- A logical sub-task is complete (e.g., one feature file fully refactored and tested) and more work remains.
- The conversation has grown long (many tool calls, many files read) and the next sub-task is a fresh scope.
- After finishing a skill that does heavy exploration (`/code-review`, `/grill-with-docs`, etc.).

When suggesting, name the next focus so the user can approve it:
> "That completes the guild settings handler. Should I create a handoff (`/handoff guild-manage`) before we start on the next file?"

Do not suggest handoff in the middle of a task (mid-function, mid-migration, mid-test). Always finish the current atomic unit first.

## Project Overview
This is a Go web application using pgx/v5 for PostgreSQL 18, sqlc for type-safe queries, goose for migrations, gomponents for HTML templating, and datastar for interactivity.

The project runs in a dev container and uses Task (Taskfile.yml) for all development commands. Never suggest running tools directly (like `goose`, `sqlc`, `air`, etc.) — always use the corresponding `task` command instead. When suggesting container commands, use Podman (not Docker) unless Docker is explicitly requested. Podman is rootless by default and uses a daemonless architecture.

## PostgreSQL 18 Best Practices
- Use PostgreSQL 18 features and syntax
- Prefer `GENERATED ALWAYS AS IDENTITY` over `SERIAL` for primary keys
- Use `TEXT` type instead of `VARCHAR` unless length constraints are needed
- Use `TIMESTAMPTZ` for datetime fields
- Add indexes on foreign keys and frequently queried columns
- Use `JSONB` (not `JSON`) for JSON data — it's faster and indexable
- Use `pg_trgm` extension for fuzzy text search when needed
- Use `uuid-ossp` or `gen_random_uuid()` for UUID generation

## Database Practices

### Connection Management
- Always use `pgxpool.Pool` for connection pooling (never single connections in production)
- Pass `*pgxpool.Pool` or the sqlc-generated interface to handlers/services
- Always use context with timeouts for database operations

### Queries with sqlc
- Write all SQL in `internal/database/queries/*.sql` files
- One SQL file per feature, named after the feature (e.g. `auth.sql`, `chat.sql`)
- Use sqlc naming conventions: `-- name: GetUser :one`, `-- name: ListUsers :many`
- Use parameterized queries (never string concatenation)
- Generated code goes in `internal/database/db/` — never edit it manually

### Tick Query Performance
- Avoid N queries inside the tick loop — each per-row query multiplies with player count
- Prefer bulk queries: collect IDs/values in Go, then issue a single `WHERE id = ANY($1)` query
- Use CTEs with `RETURNING` to decrement and collect completed rows in one round trip (see `DecrementAndList*AtZero` pattern)
- When a loop update is unavoidable, document why a bulk alternative wasn't possible

### Migrations with goose
- All migrations in `internal/database/migrations/`
- Create migrations using task commands (see Taskfile.yml)
- Always write both Up and Down migrations
- Name migrations descriptively: `YYYYMMDDHHMMSS_description.sql`
- Keep migrations small and focused
- Run with `task db:up` and `task db:down`
- Test both up and down before committing
- Never modify existing migrations that have been deployed

### CHECK Constraint Style
- **Inline** for simple single-column numeric bounds: `count INT NOT NULL CHECK (count >= 0)`
- **Named constraint** for enumeration checks and multi-column checks — likely to be altered as the domain grows:
  ```sql
  CONSTRAINT missions_action_valid CHECK (action IN ('attack', 'defend')),
  CONSTRAINT sessions_expiry_future CHECK (expires_at > created_at)
  ```
- Never mix both styles on the same column

## Project Structure

All code lives under `internal/` — nothing is intended to be imported externally.

- `cmd/server/main.go` — entry point; minimal wiring only (routes, middleware, server start)
- `internal/contextkeys/` — shared context key constants (avoids import cycles)
- `internal/routes/` — shared route path constants (avoids import cycles between `internal/ui` and feature packages)
- `internal/database/db/` — sqlc-generated code (never edit manually)
- `internal/database/migrations/` — goose migration files
- `internal/database/queries/` — SQL query files for sqlc
- `internal/email/` — email sending
- `internal/ui/` — shared gomponents: layout shell (`HomeLayout`, `KingdomLayout`), nav components, and shared UI helpers (alerts, etc.); dot-imported
- `internal/middleware/` — HTTP middleware (auth, and future: logging, CSRF, etc.)
- `internal/handlers/<feature>/` — one package per feature; HTTP handlers, page functions, and components
- `internal/router/` — router interface (avoids circular imports when injecting middleware)
- `internal/server/` — application wiring: routes, middleware registration, static file serving
- `web/static/` — static assets (CSS, JS)

### Feature Package Structure
Each feature in `internal/handlers/<feature>/` follows this pattern:
- One file exports route constants in `internal/routes/`, registers routes, and defines the `handler` struct
- Handler methods return `http.HandlerFunc` and are kept thin — read input, call queries, render response
- Content functions are pure functions returning `Node` — take only domain data as parameters, never user/path/request
- Reusable components stay in the feature package until a second package needs them, then move to `internal/ui/`

### Handlers
Handlers return `http.HandlerFunc` closures, allowing pre-computation outside the request loop:

```go
func (h *handler) handleKingdomPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
        KingdomLayout(r, "Kingdom", kingdomContent(kingdom)).Render(w)
    }
}
```

- Full-page responses call `HomeLayout(r, title, content...)` or `KingdomLayout(r, title, content...)`
- `internal/ui` is dot-imported in all handler packages
- SSE handlers use `datastar.NewSSE` — see `ui.instructions.md` for ordering rules
- Use `http.Error()` only for non-SSE error responses (before `NewSSE` is called)
- Do not re-check middleware guarantees inside handlers — if a route is behind `RequireAuth`, trust that a user is in context

### Handler Extraction Pattern
When a handler grows beyond `read input → one query → render`, extract its parts into same-package helpers. **There is no service layer** — helpers live next to the handler in the feature package. Apply this only when there's real complexity to extract; don't impose it on thin handlers.

The shape is `read signals → validate → orchestrate → render`:

**1. Sentinel errors** — exported `ErrXxx` declared next to the helpers. The error message *is* the user-facing alert text; the handler does not translate. Sentinels are the contract between validator/orchestrator and handler — that's why they're exported even when no other package imports them.

```go
var (
    ErrTargetNotFound    = errors.New("target kingdom not found")
    ErrSelfTarget        = errors.New("cannot target your own kingdom")
    ErrInsufficientUnits = errors.New("not enough units")
)
```

**2. Validators** — pure functions. Two shapes by use:

| Shape | Use when |
|---|---|
| `validateXxxInput(in *T) []error` | Multi-rule form input. Accumulates so the user sees every problem at once via `AlertError(errs...)`. |
| `validateXxx(in string) (T, error)` | Single value to parse or single invariant to check. |

Skip downstream rules whose "max" depends on an upstream field being valid (e.g. don't flag a duration error when the action itself is invalid).

**3. Orchestrators** — own the DB work, derived calculations, transaction lifecycle, and translation of `pgx.ErrNoRows` / serialization failures into sentinel errors. Return sentinels for user-visible cases; wrap infra errors with `fmt.Errorf("…: %w", err)`. Method on `*handler` when both `queries` and `pool` are needed; free function taking `db.Querier` otherwise.

**4. The handler** stays thin:

```go
func (h *handler) handleSend() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

        input := &sendInput{}
        if err := datastar.ReadSignals(r, input); err != nil { /* … */ }

        if errs := validateSendInput(input); len(errs) > 0 {
            datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errs...)))
            return
        }

        if err := h.sendCampaign(r.Context(), kingdom, input); err != nil {
            if isSendUserError(err) {
                datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
                return
            }
            log.Printf("army send: %v", err)
            datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
            return
        }

        // … render success
    }
}
```

A small `isXxxUserError(err) bool` predicate distinguishes user-visible sentinels from infra errors. Don't build a generic error→message map.

**5. Testing** — tests stay in `package <feature>` (per `testing.instructions.md`) so unexported helpers are reachable. Validators get pure table-driven tests with no stubs. Orchestrators get direct calls using the package's `stubQuerier`, no `httptest`. Handler-shell smoke tests assert the SSE response surfaces the alert text for one validator path and one orchestrator path.

See `internal/handlers/army/` for the reference implementation.

## Configuration
- Non-sensitive config (port, DSN for local dev) lives in `docker-compose.yml`
- Sensitive or environment-specific values (API keys, tokens) go in `.env` — see `.env.example` for all required keys
- Never commit `.env`; always keep `.env.example` up to date when adding new required variables
- `main.go` reads required variables at startup and calls `log.Fatal` if any are missing

### Task Runner
- Use Taskfile.yaml for ALL development tasks (required in dev container)
- `task dev` — development with live reload
- `task check` — fast compile check (`go build ./...`); run after every code change before anything else
- `task lint` — static analysis (`go vet ./...`)
- `task test` — run all tests
- `task gen` — regenerate sqlc code after query changes
- `task db:up` / `task db:down` — migrations
- Never run tools directly (goose, sqlc, air, go build, go vet, go test) — always use task commands

## Security
- Never log sensitive data (passwords, tokens, personal info)
- Use prepared statements (sqlc does this automatically)
- Validate and sanitize user input
- Use connection strings from environment variables; never commit credentials
- Use least privilege database users; enable SSL/TLS for database connections in production

## Instruction Conflicts
When a user's request contradicts something in the project instructions, always point it out explicitly before proceeding. State which instruction applies, what the user asked, and offer the choice: update the instruction, make a one-time exception, or reconsider the approach.

If something in the conversation seems worth capturing as a rule — a pattern that keeps coming up, a decision that took effort to reach, a preference that would otherwise be lost — raise it proactively.

## Language & Tool Instructions
@.github/instructions/go.instructions.md
@.github/instructions/ui.instructions.md
@.github/instructions/sql.instructions.md
@.github/instructions/testing.instructions.md
@.github/instructions/review.instructions.md
@.github/instructions/ui-test-plans.instructions.md

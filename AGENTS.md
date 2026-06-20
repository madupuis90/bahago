# Bahago

Browser-based multiplayer medieval kingdom strategy game. Go web app: pgx/v5 +
PostgreSQL 18, sqlc, goose, gomponents, datastar. Runs in a dev container;
**Task** is the only entry point to tooling.

## Read on demand

- `CONTEXT.md` — ubiquitous-language glossary. Read when naming things or
  discussing game concepts. Be precise: a *Legion* is not an *Army*, a *Kingdom*
  is not a *Player*.
- `docs/design.md` — game mechanics design notes.
- `docs/adr/` — architectural decision records (hard-to-reverse, non-obvious
  decisions). Created lazily by the `grill-with-docs` skill.
- `docs/go.md` · `docs/sql.md` · `docs/ui.md` · `docs/testing.md` — deep
  conventions. **Read the relevant one before touching that area.**
- `docs/review.md` — when doing a code review.
- `docs/ui-test-plans.md` — when running a UI test plan.
- `docs/design-system/` — visual direction + icon spec. Read when implementing
  a design handoff.
- `docs/agent-setup.md` — how the pi agent config is restored in the dev container.

## Communication & approach

- Never make up information — research or ask.
- Give reasoning behind suggestions.

## Working style (token-cost aware)

The user is quota-sensitive. Keep context lean.

- **Use `/handoff` aggressively** — between sessions and within a session when a
  logical sub-task completes and the next is a fresh scope. A precise handoff
  (exact file paths, function signatures, query names) prevents re-exploration
  after compaction; a vague one forces re-reads that cost 2–3×.
- In a new session, if the user references prior work, check `.handoff/` and
  `/pickup` before exploring.
- Separate exploration from implementation: explore early in the main context,
  then hand off focused implementation work with precise prompts.
- Surface expensive operations (broad searches, large reads) before doing them;
  default to the cheaper total-cost path.
- Proactively suggest `/handoff` when a sub-task is complete and more remains.
  Don't suggest it mid-task (mid-function, mid-migration, mid-test).
- Don't duplicate content already in artifacts (ADRs, plans, diffs, handoffs) —
  reference by path.

## Delegation workflow (cheap-tier sub-agents)

The main session runs on the expensive model (`glm-5.2`) for planning and
verification. Long, mechanical **execution** is delegated to a cheaper model
(`opencode-go/deepseek-v4-pro`) via the `subagent` tool (see
`.pi/extensions/subagent/`), which spawns an isolated `pi` subprocess so the
main context never carries the execution transcript.

**When to delegate — judgment, not automatic:**

- Delegate: execution-heavy work after planning — multi-file edits, boilerplate,
  repeated `task check` / `task test` loops, migration + query + handler triples.
- Do **not** delegate: trivial or obvious changes (a few lines, a one-line fix,
  a direct answer). The overhead of spawning a sub-agent costs more than it saves.
  When unsure, lean toward doing it in-session unless the task is clearly bulky.

**Flow:**

1. **Plan** in the main session (expensive). Produce a precise plan: exact file
   paths, function signatures, query names, acceptance criteria. This is the
   step that makes cheap-tier execution safe — the worker executes a pre-chewed
   plan, it does not evaluate requirements or design. Keep planning on the
   expensive tier; a vague plan makes the worker flail.
2. **Execute** by calling the `subagent` tool with `agent: "worker"` and the
   plan as the `task`. The worker runs on `deepseek-v4-pro:medium`, obeys the
   project conventions (it reads `AGENTS.md` + `docs/*.md` automatically), runs
   `task check` then `task test`, and reports changed files + verification
   status. It does **not** commit, push, stash, or run `task db:*`.
3. **Verify** in the main session (expensive) by reviewing `git diff` — do **not**
   re-explore the whole change. Accept the diff, or send it back to the worker
   with specific corrections.

**Safety:** the working tree is git-tracked; a gone-wrong worker is reversible
with `git checkout` / `git stash`. Never let the worker commit — keep that
boundary so the main agent owns the review point.

**Cost caveat:** a cheap model that takes 2× the turns can cost *more* than one
good turn, because each turn re-sends its growing context. If a worker is
flailing (many failed `task check` loops, re-reading files it already saw),
abort it and either give it a tighter plan or do the work in-session.

**Available agent** (`.pi/agents/`, on `deepseek-v4-pro`):

| Agent | Thinking | Tools | Purpose |
|-------|----------|-------|---------|
| `worker` | medium | all default | Executes a plan, runs `task check` + `task test` |

The `subagent` tool also supports `parallel` (array of `{agent, task}`) and
`chain` (sequential, with `{previous}` placeholder) modes. The worker agent
must not be chained to redesign its own plan — it executes, it does not replan.

## Tooling rules

- **Always use `task` commands**, never run `goose`/`sqlc`/`air`/`go build`/
  `go vet`/`go test` directly. `go doc` is a read-only lookup — run it directly.
- Container commands: use **Podman** (rootless, daemonless) unless Docker is
  explicitly requested.
- `task dev` — live reload (runs `task css:build` first)
- `task check` — fast compile check; run after every code change before anything else
- `task lint` · `task test` · `task gen` (regenerate sqlc) · `task css:build`
- `task db:up` / `task db:down` / `task db:create -- <description>`

## Project structure

All code under `internal/` — nothing importable externally.

- `cmd/server/main.go` — entry point; wiring only (routes, middleware, server
  start). Never add features here.
- `internal/contextkeys/` — shared context key constants (avoids import cycles)
- `internal/routes/` — shared route path constants (avoids import cycle between
  `internal/ui` and feature packages)
- `internal/database/{db,migrations,queries}/` — sqlc-generated code (never
  edit) / goose migrations / sqlc query files
- `internal/email/` · `internal/middleware/` · `internal/router/` · `internal/server/`
- `internal/ui/` — shared gomponents: `HomeLayout`, `KingdomLayout`, nav,
  alerts. **Dot-imported** in all handler packages.
- `internal/handlers/<feature>/` — one package per feature
- `web/static/` — served assets; `web/static/styles.css` is a **generated
  artifact** — never read or edit it. `web/static/sprite.svg` is the icon sprite.
- `web/css/` — CSS source files (edit these); run `task css:build` after any change.

### Feature package structure

- One file exports route constants in `internal/routes/`, registers routes, and
  defines the `handler` struct.
- Handler methods return `http.HandlerFunc` closures (enables pre-computation
  outside the request loop). Kept thin: read input → call queries → render.
- Content functions are pure `func(...) Node` taking only domain data, never
  user/path/request. Co-located with handlers.
- Reusable components stay in the feature package until a second package needs
  them, then move to `internal/ui/`.

### Handler pattern

- Full-page responses call `HomeLayout(r, title, content...)` or
  `KingdomLayout(r, title, content...)`; they read user/kingdom/path from
  context. No explicit `Content-Type` needed.
- SSE handlers use `datastar.NewSSE`; see `docs/ui.md` for ordering rules. Use
  `http.Error()` only before `NewSSE` is called.
- **Do not re-check middleware guarantees inside handlers.** If a route is
  behind `RequireAuth`/`LoadKingdom`, trust the context — no `if user == nil`
  guards. Defensive checks add noise and imply the scenario is possible.
- **No service layer.** When a handler grows beyond `read input → one query →
  render`, extract into same-package helpers: sentinel errors → validators →
  orchestrators → thin handler. See `internal/handlers/army/` for the reference
  implementation and `docs/testing.md` for the test shape. Apply only when
  there's real complexity; don't impose on thin handlers.

## Database

Quick rules (full conventions in `docs/sql.md`):

- `pgxpool.Pool` for pooling; context with timeouts for all DB ops.
- All SQL in `internal/database/queries/*.sql`, one file per feature. Generated
  code in `internal/database/db/` — never edit. Run `task gen` after query changes.
- Bulk ops: `WHERE id = ANY(@ids::bigint[])` or `unnest` arrays — never per-row
  queries in loops. Tick-loop queries follow the `DecrementAndList*AtZero` CTE
  pattern.
- Named params (`@name`) for 3+ params; positional (`$1`) for 1–2.
- Migrations: both Up and Down; never modify a deployed migration;
  `YYYYMMDDHHMMSS_description.sql`.
- PG18: `GENERATED ALWAYS AS IDENTITY` (not `SERIAL`), `TEXT` (not `VARCHAR`),
  `TIMESTAMPTZ`, `JSONB`. Index FKs.
- CHECK constraints: inline for single-column numeric bounds; named for
  enums/multi-column. Never mix on one column.

## Configuration

- Non-sensitive config (port, local DSN) in `docker-compose.yml`; sensitive or
  environment-specific values in `.env` (see `.env.example`). Never commit
  `.env`; keep `.env.example` in sync.
- `main.go` reads required vars at startup and `log.Fatal`s if any are missing —
  fail fast.

## Security

- Never log sensitive data. Validate/sanitize input at the HTTP boundary.
  Generic errors to clients; details server-side only. No credentials committed.

## Instruction conflicts

When a request contradicts a project instruction, **point it out explicitly
before proceeding**: name the instruction, state the request, and offer the
choice (update the instruction, one-time exception, or reconsider). If a
pattern or decision worth capturing comes up, raise it proactively — the user
prefers accurate instructions over silent divergence.

## Design handoffs

A design handoff is a self-contained package describing a UI feature to
implement. Read `docs/design-system/` first for the visual direction and icon
spec. Implement CSS in the correct `web/css/` source file, markup in the
feature handler package, run `task css:build`, and verify at real sizes.

## CSS source file map

| File | Area |
|------|------|
| `web/css/00-reset.css` | reset |
| `web/css/01-tokens.css` | design tokens |
| `web/css/02-base.css` | base element styles |
| `web/css/10-home-shell.css` | home shell: top-nav, content-area, side-nav |
| `web/css/20-shared.css` | shared: `.panel`, `.btn`, `.form-fields`, `.alert-*` |
| `web/css/30-auth.css` | auth pages |
| `web/css/31-home.css` | home page content |
| `web/css/40-kingdom-chrome.css` | kingdom chrome: topbar, bottom-nav, `.nav-stone` |
| `web/css/41-kingdom-overview.css` | overview: `.page-header`, `.overview-grid`, `.chronicle` |
| `web/css/42-allocation.css` | allocation |
| `web/css/43-buildings.css` | buildings |
| `web/css/44-units.css` | units |
| `web/css/45-world-map.css` | world map |
| `web/css/46-army.css` | army |
| `web/css/47-flipcard.css` | home/about flip card |
| `web/css/48-messages.css` | messages |
| `web/css/49-guild.css` | guild |
| `web/css/50-prayers.css` | prayers |
| `web/css/99-utilities.css` | utility overrides |

New feature sections: new numbered file between the last feature file and
`99-utilities.css`. Styles shared across 2+ features: `20-shared.css`.

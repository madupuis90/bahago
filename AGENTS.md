# Bahago

Browser-based multiplayer medieval kingdom strategy game. Go web app: pgx/v5 +
PostgreSQL 18, sqlc, goose, gomponents, datastar. Runs in a dev container;
**Task** is the only entry point to tooling.

## Read on demand

- `GLOSSARY.md` — ubiquitous-language glossary. Read when naming things or
  discussing game concepts. Be precise: a *Legion* is not an *Army*, a *Kingdom*
  is not a *Player*.
- `docs/game/rules.md` — authoritative game mechanics (the *how*). The glossary
  defines names; this defines behaviour. Draft — being authored; trust the code
  over this file until a section is filled.
- `docs/adr/` — architectural decision records (hard-to-reverse, non-obvious
  decisions). Created lazily by the `grill-with-docs` skill.
- `docs/instructions/` — deep conventions. **Read the relevant one before
  touching that area.** Files: `go.md`, `sql.md`, `ui.md`, `testing.md`,
  `review.md` (code review), `agent-setup.md` (how the pi agent config is
  restored in the dev container).
- `docs/design/` — visual direction (`ui-design.md`), icon system (`icons.md`).
  Read when implementing UI. CSS naming rules live in `docs/instructions/ui.md`.

### Do not read

`docs/human/` is the project owner's scratchpad — raw, deliberately
unstructured ideas that may contradict committed decisions or may never be
implemented. Do not load it as context or treat its contents as direction;
reading it risks importing thoughts that are not instructions.

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
- SSE handlers use `datastar.NewSSE`; see `docs/instructions/ui.md` for ordering rules. Use
  `http.Error()` only before `NewSSE` is called.
- **Do not re-check middleware guarantees inside handlers.** If a route is
  behind `RequireAuth`/`LoadKingdom`, trust the context — no `if user == nil`
  guards. Defensive checks add noise and imply the scenario is possible.
- **No service layer.** When a handler grows beyond `read input → one query →
  render`, extract into same-package helpers: sentinel errors → validators →
  orchestrators → thin handler. See `internal/handlers/army/` for the reference
  implementation and `docs/instructions/testing.md` for the test shape. Apply only when
  there's real complexity; don't impose on thin handlers.

## Database

Quick rules (full conventions in `docs/instructions/sql.md`):

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
| `web/css/40-kingdom-chrome.css` | kingdom chrome: topbar, bottom-nav, gems, crest, pills |
| `web/css/41-kingdom-overview.css` | overview: `.page-header`, `.overview-grid`, combat-log |
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

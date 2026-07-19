---
description: "Use when performing a code review"
---

# Code Review Workflow

## Process

1. Run `git log --oneline` to identify the commits belonging to the feature
2. Run `git diff <first-commit>^..HEAD` to pull the full diff
3. Use the diff as the primary source. Read a full file only when the diff lacks context — for example, to understand an unchanged caller, a type definition, or surrounding logic. Do not read every changed file in full by default.
4. Present a structured review: **what needs to change** (with specific file/line references), and **any open questions**
5. Work through each issue with the user one at a time — do not batch-fix everything at once

> Note: this project commits directly to main (solo dev, one feature at a time). If branches are introduced in future, use `git diff main...HEAD` instead.

## Checklist

Apply all active instructions. The items below are the most commonly violated — check each one explicitly.

### SQL & Database
- [ ] All SQL is in `internal/database/queries/*.sql`, not inline in Go
- [ ] New query files follow the one-file-per-feature convention
- [ ] Bulk operations use `ANY(@ids::bigint[])` or `unnest` — no per-row queries in loops
- [ ] Queries with 3+ parameters use named parameters (`@param`) not positional (`$1`)
- [ ] Atomic check-and-insert uses a CTE guard, not separate select + insert
- [ ] Tick loop queries follow the `DecrementAndList*AtZero` pattern where applicable
- [ ] New migrations have both Up and Down sections
- [ ] Primary keys use `GENERATED ALWAYS AS IDENTITY`
- [ ] Datetime columns use `TIMESTAMPTZ`
- [ ] Text columns use `TEXT` unless length enforcement is required
- [ ] CHECK constraints follow inline vs. named style rules (see `sql.md`)

### Go
- [ ] No `package` declaration duplicated in edited files
- [ ] Errors wrapped with `fmt.Errorf("context: %w", err)`
- [ ] No error both logged and returned
- [ ] `errors.AsType[T]()` used instead of `var target T; errors.As(err, &target)` (Go 1.26+)
- [ ] `any` used instead of `interface{}`
- [ ] Handler functions return `http.HandlerFunc` closures (not bare functions)
- [ ] No defensive nil checks for values guaranteed by middleware (e.g., user/kingdom in context)
- [ ] No `http.Error()` called after `datastar.NewSSE` (SSE responses only)

### UI / Templates
- [ ] No string literals for **route paths** — use `internal/routes/` constants
- [ ] Element IDs use inline string literals (no `const`) and only when targeted by SSE or CSS
- [ ] Signal name strings kept adjacent in one file (the `json` tag + the `ds.*` call); no shared signal consts
- [ ] `Class()` and `Classes{}` not mixed on the same element
- [ ] `Iff` used (not `If`) when condition guards a nil pointer dereference
- [ ] SSE patches use `PatchElementGostar` / `MarshalAndPatchSignals` (the project's datastar-go API) — no `MergeFragments`/`MergeSignals`
- [ ] Full-page responses use `HomeLayout` or `KingdomLayout` — no raw `HTML5()`
- [ ] Alert components use `AlertError`/`AlertSuccess`/`AlertContainer` from `internal/ui/` — no per-feature reimplementations

### Structure & Conventions
- [ ] New files placed in the correct package (`internal/handlers/<feature>/`, `internal/routes/`, etc.)
- [ ] Route path constants defined in `internal/routes/routes.go`
- [ ] Reusable components stay in the feature package until a second package needs them, then move to `internal/ui/`
- [ ] No features added to `main.go` beyond wiring (routes, middleware, server start)
- [ ] `task gen` run after any SQL query change
- [ ] `task test` passes

### Security
- [ ] No sensitive data logged (passwords, tokens, personal info)
- [ ] User input validated at HTTP boundary before use
- [ ] No credentials committed or hardcoded
- [ ] Generic error messages returned to clients; details logged server-side only

---
description: "Use when writing or reviewing SQL query files in internal/database/queries/ or migrations in internal/database/migrations/"
applyTo: "**/database/queries/*.sql,**/database/migrations/*.sql"
---

# SQL Conventions

This project uses **sqlc** for type-safe query generation and **goose** for migrations. All SQL targets **PostgreSQL 18**.

## Query Files

- One file per feature, named after the feature: `auth.sql`, `campaigns.sql`, `units.sql`
- All queries live in `internal/database/queries/` — never write raw SQL in Go
- Generated code goes in `internal/database/db/` — never edit it manually
- After any change to a query file, run `task gen` to regenerate

## sqlc Naming Conventions

- Annotation format: `-- name: QueryName :one|:many|:exec|:execrows`
- Use `:one` for single-row results, `:many` for lists, `:exec` for writes with no return, `:execrows` when you need the affected row count
- Name queries as `VerbNoun` in PascalCase: `GetUserByEmail`, `CreateSession`, `ListActiveKingdoms`
- Use named parameters (`@param_name`) instead of positional (`$1`) when a query has three or more parameters — it improves readability in both SQL and generated Go
- Use positional parameters (`$1`, `$2`) for one or two parameters where the meaning is obvious from context

## Query Design

- Use parameterized queries always — sqlc enforces this, never use string concatenation
- Keep queries simple and readable — one clear intent per query
- Prefer a single well-structured query over multiple round trips
- Use `WHERE id = ANY(@ids::bigint[])` for bulk lookups — never loop individual queries per row
- Use CTEs with `RETURNING` to decrement-and-collect in one round trip (see `DecrementAndList*AtZero` pattern in `campaigns.sql`)
- Atomic check-and-insert: use a CTE (`WITH available AS (SELECT ...)`) to guard inserts rather than separate select + insert — prevents TOCTOU races
- `LIMIT 1` is only needed for `SELECT * FROM table WHERE condition` style queries where the index does not already guarantee uniqueness

## Bulk Operations Pattern

When updating multiple rows with different values per row, use `unnest` arrays:

```sql
-- name: BulkUpdateCampaignCounts :exec
UPDATE kingdom_campaigns
SET count = data.count
FROM (
    SELECT unnest(@ids::bigint[]) AS id,
           unnest(@counts::int[]) AS count
) AS data
WHERE kingdom_campaigns.id = data.id;
```

When performing the same update on many rows, use `ANY`:

```sql
-- name: BulkActivateCampaigns :exec
UPDATE kingdom_campaigns
SET status = 'active', ticks_remaining = action_ticks
WHERE id = ANY(@ids::bigint[]);
```

## Migration Files

- Location: `internal/database/migrations/`
- Create with: `task db:create -- description_here` (produces a timestamped file)
- Always write both Up and Down sections
- Keep migrations small and focused — one schema change per file
- Never modify a migration that has already been applied to any environment
- Run `task db:up` / `task db:down` — never call goose directly

### Column Type Defaults

| Use case | Type |
|----------|------|
| Primary key | `GENERATED ALWAYS AS IDENTITY` |
| Text | `TEXT` (not `VARCHAR` unless length enforcement is needed) |
| Datetime | `TIMESTAMPTZ` |
| JSON data | `JSONB` (not `JSON`) |
| UUID | `gen_random_uuid()` |

### CHECK Constraint Style

- **Inline** for simple single-column numeric bounds:
  ```sql
  count INT NOT NULL CHECK (count >= 0)
  ```
- **Named constraint** for enumeration checks and multi-column checks:
  ```sql
  CONSTRAINT missions_action_valid CHECK (action IN ('attack', 'defend')),
  CONSTRAINT sessions_expiry_future CHECK (expires_at > created_at)
  ```
- Never mix both styles on the same column

### Index Guidelines

- Always index foreign key columns
- Add indexes on columns used in frequent `WHERE` filters
- Use `pg_trgm` extension for fuzzy text search when needed

# GitHub Copilot Instructions for Project MAD

## Communication & Approach
- Act as a senior developer
- Explain concepts clearly and provide context for decisions
- Never make up information - if uncertain, research or ask for clarification
- Always verify best practices for the specific version of tools/frameworks being used
- Research current recommendations before suggesting patterns or approaches
- Provide reasoning behind suggestions to help build understanding

## Project Overview
This is a Go web application using pgx/v5 for PostgreSQL 18, sqlc for type-safe queries, goose for migrations, and gomponents for HTML templating.

The development environment runs on Bazzite Linux using Podman as the default container runtime. Bazzite is a Fedora-based immutable/atomic desktop OS that is container-native. As an immutable system, development work should be done inside containers rather than installing tools on the host system.

The project runs in a dev container and uses Task (Taskfile.yml) for all development commands. Never suggest running tools directly (like `goose`, `sqlc`, etc.) - always use the corresponding `task` command instead. When suggesting container commands, use Podman (not Docker) unless Docker is explicitly requested. Podman is rootless by default and uses a daemonless architecture, making it the preferred container runtime for Bazzite.

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

### Code Style
- Follow standard Go formatting (use `gofmt`)
- Use tabs for indentation
- Keep lines under 100 characters when reasonable
- Use meaningful variable names (avoid single letters except in short scopes like loops)
- Group imports: stdlib, external packages, internal packages
- Add comments for exported functions, types, and packages

### Error Handling
- Always handle errors explicitly - never ignore them
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors rather than panicking (except in init or unrecoverable situations)
- Use `errors.Is()` and `errors.As()` for error checking
- Check errors immediately after the function call

### Functions and Methods
- Keep functions small and focused on a single responsibility
- Prefer returning errors over panic
- Use named return values sparingly (only when it improves clarity)
- Accept interfaces, return concrete types when possible
- Use context.Context as the first parameter for functions that might block or need cancellation

### Concurrency
- Always pass context to goroutines that might need cancellation
- Use channels for communication, mutexes for state protection
- Close channels from the sender side only
- Always handle goroutine cleanup to prevent leaks

## Database Practices

### Connection Management
- Always use `pgxpool.Pool` for connection pooling (never single connections in production)
- Pass `*pgxpool.Pool` or the sqlc-generated interface to handlers/services
- Set appropriate pool size based on application needs
- Always use context with timeouts for database operations

### Queries with sqlc
- Write all SQL in `internal/database/queries/*.sql` files
- Use sqlc naming conventions: `-- name: GetUser :one`, `-- name: ListUsers :many`
- Keep queries simple and readable
- Use parameterized queries (never string concatenation)
- Generated code goes in `internal/database/` - never edit it manually

### Migrations with goose
- All migrations in `internal/database/migrations/`
- Create migrations using task commands (see Taskfile.yml for available commands)
- Always write both Up and Down migrations
- Name migrations descriptively: `YYYYMMDDHHMMSS_description.sql`
- Keep migrations small and focused
- Run migrations with `task db:up` and `task db:down`
- Test migrations both up and down before committing
- Never modify existing migrations that have been deployed

## Project Structure

### Directory Organization
- `cmd/server/` - Application entry point (main.go)
- `internal/database/` - Database code (sqlc generated + migrations)
- `internal/handlers/` - HTTP handlers
- `internal/ui/` - Gomponents templates (components, layouts, pages)
- Keep main.go minimal - just wiring and server setup

### Internal Packages
- Code in `internal/` cannot be imported by external projects
- Organize by feature/domain when the project grows
- Avoid circular dependencies between internal packages

## Web/HTTP Practices

### Handlers
- Handlers should be thin - delegate to service layer for business logic
- Return appropriate HTTP status codes
- Always set Content-Type headers
- Use http.Error() for error responses
- Log errors before returning them to clients

### Gomponents
- Import with dot notation for cleaner syntax: `import . "maragu.dev/gomponents/html"`
- This allows components to look like HTML: `Div()`, `H1()`, `P()` instead of `html.Div()`
- Reuse components - keep them in `internal/ui/components/`
- Create layouts for common page structure in `internal/ui/layouts/`
- Keep page-specific templates in `internal/ui/pages/`
- Components should be pure functions returning `gomponents.Node`

## Configuration

### Environment Variables
- Environment variables are set in docker-compose file for local development
- Document all required environment variables in README.md
- Provide sensible defaults where possible
- Use `os.Getenv()` or a config package for reading environment variables

### Task Runner
- Use Taskfile.yml for ALL development tasks (required in dev container)
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

## Dependencies

### Adding Dependencies
- Use `go get` to add dependencies
- Run `go mod tidy` to clean up go.mod
- Review dependencies before adding them (check maintenance, license, size)
- Prefer standard library over external dependencies when reasonable

### Tools
- Install tools using `go install` or as `tool` directives in go.mod
- Current tools: air (live reload), goose (migrations), sqlc (query generation)

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

## Performance

### Optimization Guidelines
- Profile before optimizing
- Use connection pooling (pgxpool)
- Add database indexes for frequently queried columns
- Use `LIMIT` in queries when appropriate
- Consider caching for expensive queries (but measure first)

## Documentation

### Code Comments
- Document exported types, functions, and constants
- Explain "why" not "what" in comments
- Keep comments up to date with code changes
- Use `// TODO:` for known issues or future improvements

### README
- Keep README.md updated with setup instructions
- Document environment variables
- Include examples of common operations
- Add troubleshooting section for common issues

## Commit Guidelines

### Git Commits
- Write clear commit messages
- Use conventional commits format when possible (feat:, fix:, docs:, etc.)
- Keep commits focused on a single change
- Test before committing

### What Not to Commit
- `.env` files with secrets
- Generated binaries (`tmp/`, `bin/`)
- IDE-specific files (add to .gitignore)
- `go.sum` changes should be committed when dependencies change

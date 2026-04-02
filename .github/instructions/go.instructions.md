---
description: "Instructions for writing Go code following idiomatic Go practices and community standards"
applyTo: "**/*.go,**/go.mod,**/go.sum"
---

# Go Development Instructions

Follow idiomatic Go practices based on [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), and [Google's Go Style Guide](https://google.github.io/styleguide/go/).

This project uses **Go 1.25**.

## General Style

- Write simple, clear, and idiomatic Go — favor clarity over cleverness
- Keep the happy path left-aligned: return early to reduce nesting
- Prefer `if condition { return }` over else blocks
- Make the zero value useful
- Write self-documenting code with clear, descriptive names
- Leverage the standard library before reaching for external packages

## Naming Conventions

### Packages
- Lowercase, single-word names — no underscores, hyphens, or mixedCaps
- Describe what the package provides, not what it contains
- Avoid generic names like `util`, `common`, or `base`
- Singular, not plural

### Package Declarations (CRITICAL)
- **Each Go file must have exactly ONE `package` declaration**
- When editing an existing file: **preserve** the existing declaration, never add another
- When creating a new file: check what package name other `.go` files in the same directory use and match it
- If it's a new directory, use the directory name as the package name
- **Duplicate `package` declarations are a compile error** — always verify before writing

### Variables and Functions
- Use mixedCaps (camelCase) — not underscores
- Short but descriptive names; single-letter variables only in very short scopes
- Exported names start with a capital letter; unexported with lowercase
- Avoid stuttering: prefer `http.Server` over `http.HTTPServer`

### Interfaces
- Use `-er` suffix when possible: `Reader`, `Writer`, `Formatter`
- Single-method interfaces named after the method
- Keep interfaces small (1–3 methods ideal)
- Define interfaces close to where they're used, not where they're implemented
- Don't export interfaces unless necessary

### Constants
- MixedCaps for exported, mixedCaps for unexported
- Group related constants in `const` blocks
- Use typed constants for better type safety

## Formatting

- Always use `gofmt`
- Group imports: stdlib → external packages → internal packages
- Add blank lines to separate logical groups of code

## Comments

- Prioritize self-documenting code — comment only when explaining complex logic, business rules, or non-obvious behavior
- All exported symbols must have doc comments starting with the symbol name
- Write comments in complete sentences
- Comment the *why*, not the *what*
- No emoji in code or comments

## Error Handling

- Always handle errors explicitly — never ignore with `_` without a documented reason
- Check errors immediately after the call
- Wrap with context: `fmt.Errorf("context: %w", err)`
- Use `errors.Is()` and `errors.As()` for checking
- Error variables named `err`; error messages lowercase with no trailing punctuation
- Return errors rather than panicking (except init or truly unrecoverable situations)
- Don't both log and return an error — choose one

## Functions and Methods

- Keep functions small and focused on a single responsibility
- Use `context.Context` as the first parameter for functions that may block or need cancellation
- Accept interfaces, return concrete types
- Named return values only when they genuinely improve clarity
- Use pointer receivers for large structs or mutation; value receivers for small structs

## Interfaces and Types

- Use `any` instead of `interface{}` (Go 1.18+)
- Prefer specific types or generic type parameters with constraints over unconstrained `any`
- Use type assertions carefully and always check the second return value
- Use struct tags for JSON, database mappings, etc.

## Concurrency

- Use channels for communication; mutexes for protecting shared state
- Close channels from the sender side only
- Always know how a goroutine will exit — avoid goroutine leaks
- Use `sync.WaitGroup.Go` (Go 1.25):
  ```go
  var wg sync.WaitGroup
  wg.Go(task1)
  wg.Go(task2)
  wg.Wait()
  ```
- Use `sync.RWMutex` when reads significantly outnumber writes
- Use `sync.Once` for one-time initialization

## HTTP Handlers

- Use the enhanced `net/http` `ServeMux` with method+pattern routing (Go 1.22+):
  ```go
  mux.HandleFunc("GET /users/{id}", handler)
  ```
- Handlers should be thin — delegate business logic to a service/query layer
- Return appropriate HTTP status codes
- Use `http.Error()` for error responses
- Log errors server-side before returning generic messages to clients

## Testing

- Table-driven tests for multiple scenarios
- Name tests: `TestFunctionName_Scenario`
- Use subtests with `t.Run` for organisation
- Mark helpers with `t.Helper()`
- Clean up with `t.Cleanup()`
- Keep tests in the same package; use `_test` suffix package only for black-box testing
- Test both success and error paths

## Struct Literals as Arguments

When passing a multi-field struct literal as the sole argument to a function call inside an `if err :=` clause, assign it to a named variable first — it avoids deep nesting and makes both the struct construction and the error check easier to read:

```go
// Good
params := db.CreateThingParams{
    Name:      name,
    UserID:    userID,
    ExpiresAt: time.Now().Add(24 * time.Hour),
}
if err := h.queries.CreateThing(ctx, params); err != nil {

// Avoid
if err := h.queries.CreateThing(ctx, db.CreateThingParams{
    Name:      name,
    UserID:    userID,
    ExpiresAt: time.Now().Add(24 * time.Hour),
}); err != nil {
```

## Common Pitfalls

- Not checking errors
- Goroutine leaks (always ensure goroutines can exit)
- Not closing resources — use `defer` for files, connections, response bodies
- Modifying maps concurrently
- Misunderstanding nil interfaces vs nil pointers
- Global variables when dependency injection is cleaner
- Duplicate `package` declarations — always check before adding one

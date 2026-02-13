# Bahago

Go web application with type-safe PostgreSQL queries and live reload.

## Stack

- **Go 1.25** - Backend language
- **PostgreSQL 18** - Database
- **pgx/v5** - PostgreSQL driver with connection pooling
- **sqlc** - Type-safe SQL query generation
- **goose** - Database migrations
- **gomponents** - HTML templating in Go
- **air** - Live reload for development
- **Task** - Task runner (replaces Makefile)

## Prerequisites

This project uses **Dev Containers**. You'll need:
- Podman
- VS Code with Dev Containers extension

## Quick Start

```bash
# Start development server with live reload
task dev

# Run migrations
task db:up

# Generate code from SQL queries
task gen
```

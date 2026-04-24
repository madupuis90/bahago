FROM mcr.microsoft.com/devcontainers/go:1.26-trixie

# Install PostgreSQL client tools (psql, pg_dump, etc.)
RUN apt-get update && apt-get install -y --no-install-recommends postgresql-client && rm -rf /var/lib/apt/lists/*

# Need to make sure named volume are created with proper permission otherwise podman creates them with root
RUN   mkdir -p /home/vscode/.cache /go/pkg \
  &&  chown -R 1000:1000 /home/vscode /go/pkg
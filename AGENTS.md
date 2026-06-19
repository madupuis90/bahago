# Agent Setup Notes

This file documents how the coding-agent tooling (pi) is configured inside the
dev container. It is housekeeping, not game-domain guidance — domain
conventions live in `CLAUDE.md`.

## Why this exists

The dev container's `/home` (and therefore `~/.pi/agent/`) is wiped whenever the
container is recreated. The repo checkout at `/workspaces/bahago` survives, so
agent config is stored there and restored on container creation.

## Restore flow

`devcontainer.json` runs `.devcontainer/setup-pi.sh` in `postCreateCommand`,
which copies the following into `~/.pi/agent/`:

| Source (in repo) | Restores to | Tracked? |
|---|---|---|
| `.devcontainer/configs/pi-settings.json` | `~/.pi/agent/settings.json` | yes |
| `.devcontainer/configs/pi-keybindings.json` | `~/.pi/agent/keybindings.json` | yes |
| `.devcontainer/configs/pi-auth.json` | `~/.pi/agent/auth.json` | **no** (gitignored — contains API key) |

`pi-auth.json` is gitignored. Copy `.devcontainer/configs/pi-auth.example.json`
to `pi-auth.json` and fill in your key, or run `/login` after starting pi. The
setup script is guarded so a fresh clone without `pi-auth.json` warns instead of
failing.

No custom `models.json` is restored — built-in providers (e.g. `opencode-go`)
are used directly.

## Project settings

`.pi/settings.json` (committed) sets:

- `sessionDir` → `/workspaces/bahago/.pi/sessions` so **session history survives
  container recreation** (`.pi/sessions/` is gitignored).
- `compaction.keepRecentTokens` → a generous recent slice kept verbatim during
  auto-compaction, suited to long multi-file sessions.
- `enabledModels` → the two-model cycle used by the toggle hotkey (see below).

## Keybindings

`~/.pi/agent/keybindings.json` is restored from
`.devcontainer/configs/pi-keybindings.json` and remaps model cycling off the
VSCode-intercepted `Ctrl+P` / `Shift+Ctrl+P`:

- `Ctrl+Shift+M` — cycle forward through scoped models (acts as a toggle when
  two models are scoped)
- `Ctrl+Alt+M` — cycle backward

The scoped model set comes from `enabledModels` in `.pi/settings.json`. Edit it
there, or use `/scoped-models` interactively and press `Ctrl+S` to save.

## Project trust

Because `.pi/settings.json` exists, pi prompts once to trust this folder. The
saved decision lives in `~/.pi/agent/trust.json` and is **not** restored by the
setup script — that's intentional, so a fresh machine still asks. Approve and
run `/trust` to silence it for future sessions.

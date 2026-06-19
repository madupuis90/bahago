#!/bin/bash
set -e

echo "📦 Installing global Pi Coding Agent..."
npm install -g --ignore-scripts @earendil-works/pi-coding-agent

echo "🤖 Configuring Pi Agent..."
mkdir -p ~/.pi/agent

# Settings are non-sensitive and tracked in the repo.
# (No custom providers/models are needed; opencode-go is a built-in provider.)
cp .devcontainer/configs/pi-settings.json    ~/.pi/agent/settings.json
cp .devcontainer/configs/pi-keybindings.json ~/.pi/agent/keybindings.json

# Auth contains API keys and is gitignored — copy only if present so a
# fresh clone without pi-auth.json doesn't fail the postCreateCommand.
if [ -f .devcontainer/configs/pi-auth.json ]; then
  cp .devcontainer/configs/pi-auth.json ~/.pi/agent/auth.json
  echo "🔑 Pi auth restored from .devcontainer/configs/pi-auth.json"
else
  echo "⚠️  No .devcontainer/configs/pi-auth.json found."
  echo "    Copy .devcontainer/configs/pi-auth.example.json to pi-auth.json and fill in your key,"
  echo "    or run /login after starting pi."
fi

echo "✅ Pi Agent configured."

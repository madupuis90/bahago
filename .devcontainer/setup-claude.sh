#!/bin/bash
set -e

echo "🤖 Configuring Claude Code..."
mkdir -p ~/.claude
cp .devcontainer/configs/claude-settings.json ~/.claude/settings.json
echo "✅ Claude Code configured."
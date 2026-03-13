#!/bin/bash
# StarClaw Claw — Deploy via GitHub
# Usage: bash scripts/deploy.sh [commit message] [target]
#   target: api (default), web, all
#
# Flow: sync to OSS repo → push GitHub → server git pull → docker rebuild
set -e

MSG="${1:-update}"
TARGET="${2:-api}"
REMOTE_HOST="root@starclaw.me"

echo "=== [1/3] Sync to OSS repo ==="
# Copy to starclaw-oss (excluding binaries, node_modules, data)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OSS_DIR="$(cd "$PROJECT_DIR/../../starclaw-oss" 2>/dev/null && pwd || echo "")"

if [ -n "$OSS_DIR" ] && [ -d "$OSS_DIR/.git" ]; then
    rsync -a --delete \
        --exclude='node_modules' --exclude='.git' --exclude='data' \
        --exclude='*.tar.gz' --exclude='*.exe' --exclude='mcp-bridge-*' \
        --exclude='dist' --exclude='.vite' --exclude='server.exe' \
        "$PROJECT_DIR/" "$OSS_DIR/"
    cd "$OSS_DIR"
    git add -A
    git diff --cached --quiet || git commit -m "$MSG"
    git push origin main
    echo "Pushed to GitHub"
else
    echo "OSS repo not found, pushing from current repo"
    cd "$PROJECT_DIR"
    git add -A
    git diff --cached --quiet || git commit -m "$MSG"
    git push origin main
fi

echo ""
echo "=== [2/3] Deploy on server (target: $TARGET) ==="
ssh "$REMOTE_HOST" "bash /opt/starclaw/deploy-update.sh $TARGET"

echo ""
echo "=== [3/3] Done ==="
echo "https://app.starclaw.me"

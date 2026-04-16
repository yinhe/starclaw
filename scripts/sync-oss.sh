#!/bin/bash
# StarClaw 🦞 Open-Source Sync Script
# Syncs claw/ (open-source) from private monorepo to github.com/yinhe/starclaw
#
# Usage: bash claw/scripts/sync-oss.sh [commit message]
#
# Syncs:    claw/ contents → OSS repo root
# Excludes: queen/, overlord/ (closed-source)
# Note: README.md, LICENSE, .env.example, .gitignore are sourced from claw/ (OSS versions)

set -e

OSS_REPO="git@github.com:yinhe/starclaw.git"
OSS_DIR="/tmp/claw-oss"
# Monorepo root is two levels up: scripts/ → claw/ → starclaw/
MONOREPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
CLAW_DIR="$MONOREPO_DIR/claw"

COMMIT_MSG="${1:-sync: update from monorepo $(date +%Y-%m-%d)}"

echo "=========================================="
echo "  StarClaw 🦞 Open-Source Sync"
echo "=========================================="
echo "  Source:  $CLAW_DIR"
echo "  Target:  $OSS_REPO"
echo "=========================================="

# Clone or pull the OSS repo
if [ -d "$OSS_DIR/.git" ]; then
  echo "[1/3] Pulling latest OSS repo..."
  cd "$OSS_DIR" && git pull --rebase
else
  echo "[1/3] Cloning OSS repo..."
  rm -rf "$OSS_DIR"
  git clone "$OSS_REPO" "$OSS_DIR"
fi

# Sync claw/ contents to OSS repo root
echo "[2/3] Syncing claw/ → OSS repo..."
rsync -av --delete \
  --exclude='node_modules' \
  --exclude='.gradle' \
  --exclude='build' \
  --exclude='.dart_tool' \
  --exclude='data' \
  --exclude='scripts/sync-oss.sh' \
  "$CLAW_DIR/" "$OSS_DIR/"

# README, LICENSE, .env.example, .gitignore, docker-compose are already in claw/
# and synced by rsync above. No need to copy from monorepo root.

# Commit and push
echo "[3/3] Committing and pushing..."
cd "$OSS_DIR"
git add -A
if git diff --cached --quiet; then
  echo "  No changes to commit."
else
  git commit -m "$COMMIT_MSG"
  git push origin main
  echo "  ✅ Pushed to $OSS_REPO"
fi

echo ""
echo "=========================================="
echo "  Sync complete!"
echo "=========================================="

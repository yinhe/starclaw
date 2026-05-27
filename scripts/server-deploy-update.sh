#!/bin/bash
# server-deploy-update.sh — Claw deploy script for Server A
# Called by Nydus post-receive hook after code is synced to /opt/starclaw/
#
# Usage:
#   bash scripts/server-deploy-update.sh all --no-pull
#   bash scripts/server-deploy-update.sh api
#   bash scripts/server-deploy-update.sh lite
#   bash scripts/server-deploy-update.sh web
#
# Targets:
#   all   — rebuild api + lite + web, restart containers, trigger Hive upgrade
#   api   — rebuild and restart starclaw-api only
#   lite  — rebuild lite image and trigger Hive rolling upgrade
#   web   — rebuild and restart starclaw-web only
set -euo pipefail

DEPLOY_DIR="/opt/starclaw"
COMPOSE_FILE="docker-compose.yml"
HIVE_URL="http://localhost:9090"
LOG_PREFIX="[claw-deploy]"

cd "$DEPLOY_DIR"

TARGET="${1:-all}"
NO_PULL=false
for arg in "$@"; do
  [ "$arg" = "--no-pull" ] && NO_PULL=true
done

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $LOG_PREFIX $*"; }

build_api() {
  log "Building starclaw-api image..."
  docker compose -f "$COMPOSE_FILE" build --no-cache api
  log "Restarting starclaw-api container..."
  docker compose -f "$COMPOSE_FILE" up -d api
  log "starclaw-api deployed ✓"
}

build_web() {
  log "Building starclaw-web image..."
  docker compose -f "$COMPOSE_FILE" build --no-cache web
  log "Restarting starclaw-web container..."
  docker compose -f "$COMPOSE_FILE" up -d web
  log "starclaw-web deployed ✓"
}

build_lite() {
  log "Building starclaw-claw-lite image..."
  docker build -t starclaw-claw-lite:latest -f "$DEPLOY_DIR/Dockerfile.lite" "$DEPLOY_DIR/"
  log "starclaw-claw-lite image rebuilt ✓"

  # Trigger Hive rolling upgrade for all lite containers
  log "Triggering Hive rolling upgrade..."
  hive_result=$(curl -s -w '\n%{http_code}' -X POST "$HIVE_URL/hive/admin/upgrade-instances" \
    -H 'Content-Type: application/json' -d '{"image":"starclaw-claw-lite:latest"}' 2>&1 || true)
  hive_code=$(echo "$hive_result" | tail -1)
  hive_body=$(echo "$hive_result" | head -n -1)

  if [ "$hive_code" = "200" ] || [ "$hive_code" = "204" ]; then
    log "Hive upgrade triggered ✓ (HTTP $hive_code)"
  else
    log "WARNING: Hive upgrade returned HTTP $hive_code: $hive_body"
    log "Lite containers may need manual restart"
  fi
}

cleanup() {
  log "Pruning Docker build cache..."
  docker builder prune -f 2>/dev/null || true
  log "Cleanup done ✓"
}

# Main
log "Deploy target=$TARGET no_pull=$NO_PULL"

case "$TARGET" in
  all)
    build_api
    build_lite
    build_web
    cleanup
    ;;
  api)
    build_api
    cleanup
    ;;
  lite)
    build_lite
    cleanup
    ;;
  web)
    build_web
    cleanup
    ;;
  *)
    echo "Usage: $0 {all|api|lite|web} [--no-pull]"
    exit 1
    ;;
esac

# Final health check
log "Health check..."
api_health=$(curl -s http://localhost:8080/health 2>/dev/null || echo '{"status":"unreachable"}')
hive_health=$(curl -s "$HIVE_URL/hive/health" 2>/dev/null || echo '{"status":"unreachable"}')
log "API:  $api_health"
log "Hive: $hive_health"
log "Deploy complete ✓"

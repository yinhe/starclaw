#!/bin/bash
# StarClaw Server Deploy Script
# Usage: bash deploy-update.sh [api|web|all] [--no-pull] [--rev=abc1234]
# Default: all (rebuild both api and web)
#
# Supports:
#   - Nydus-triggered deploys (code already synced, use --no-pull)
#   - GitHub-triggered deploys (git pull)
#   - Rollback via BUILD_VERSION env
set -e

TARGET="${1:-all}"
NO_PULL=false
REV=""

for arg in "$@"; do
  case "$arg" in
    --no-pull) NO_PULL=true ;;
    --rev=*)   REV="${arg#--rev=}" ;;
  esac
done

cd /opt/starclaw
DEPLOY_START=$(date +%s)
echo "╔══════════════════════════════════════════╗"
echo "║  StarClaw Deploy — $(date '+%Y-%m-%d %H:%M:%S')  ║"
echo "╚══════════════════════════════════════════╝"

echo ""
echo "=== [1/4] Source code ==="
if [ "$NO_PULL" = true ]; then
  echo "  Nydus mode: code already synced"
else
  git pull origin main
fi

# Capture version
if [ -n "$REV" ]; then
  BUILD_VER="$REV"
elif [ -f .git/HEAD ]; then
  BUILD_VER=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")
else
  BUILD_VER="dev"
fi
echo "$BUILD_VER" > api/.version
echo "  Version: $BUILD_VER"

echo ""
echo "=== [2/4] Building ($TARGET) ==="
export BUILD_VERSION="$BUILD_VER"
case "$TARGET" in
  api)
    docker compose -f docker-compose.prod.yml build api
    docker compose -f docker-compose.prod.yml up -d api
    ;;
  web)
    docker compose -f docker-compose.prod.yml build web
    docker compose -f docker-compose.prod.yml up -d web
    ;;
  all)
    docker compose -f docker-compose.prod.yml build api web
    docker compose -f docker-compose.prod.yml up -d api web
    ;;
  *)
    echo "Unknown target: $TARGET (use api, web, or all)"
    exit 1
    ;;
esac

echo ""
echo "=== [3/4] Health check ==="
RETRIES=0
MAX_RETRIES=6
API_OK=false
WEB_OK=false

while [ $RETRIES -lt $MAX_RETRIES ]; do
  sleep 5
  RETRIES=$((RETRIES + 1))

  if [ "$TARGET" = "api" ] || [ "$TARGET" = "all" ]; then
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
      API_OK=true
    fi
  fi

  if [ "$TARGET" = "web" ] || [ "$TARGET" = "all" ]; then
    if curl -sf -o /dev/null http://localhost:8081/ 2>&1; then
      WEB_OK=true
    fi
  fi

  # Check if all required services are healthy
  if [ "$TARGET" = "api" ] && [ "$API_OK" = true ]; then break; fi
  if [ "$TARGET" = "web" ] && [ "$WEB_OK" = true ]; then break; fi
  if [ "$TARGET" = "all" ] && [ "$API_OK" = true ] && [ "$WEB_OK" = true ]; then break; fi

  echo "  Retry $RETRIES/$MAX_RETRIES..."
done

echo ""
echo "=== [4/4] Result ==="
DEPLOY_END=$(date +%s)
DURATION=$((DEPLOY_END - DEPLOY_START))
FAIL=false

if [ "$TARGET" = "api" ] || [ "$TARGET" = "all" ]; then
  if [ "$API_OK" = true ]; then echo "  ✅ API healthy"; else echo "  ❌ API FAILED"; FAIL=true; fi
fi
if [ "$TARGET" = "web" ] || [ "$TARGET" = "all" ]; then
  if [ "$WEB_OK" = true ]; then echo "  ✅ Web healthy"; else echo "  ❌ Web FAILED"; FAIL=true; fi
fi

echo ""
echo "  Version:  $BUILD_VER"
echo "  Target:   $TARGET"
echo "  Duration: ${DURATION}s"

if [ "$FAIL" = true ]; then
  echo ""
  echo "  ⚠️  Deploy completed with failures!"
  echo "  Check logs: docker compose -f docker-compose.prod.yml logs --tail=50"
  exit 1
fi

echo ""
echo "  🦞 Deploy complete! https://app.starclaw.me"

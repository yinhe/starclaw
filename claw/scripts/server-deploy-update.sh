#!/bin/bash
# StarClaw Server Deploy Script
# Usage: bash deploy-update.sh [api|web|all]
# Default: all (rebuild both api and web)
set -e

TARGET="${1:-all}"
cd /opt/starclaw

echo "=== [1/3] Pulling latest code ==="
git pull origin main

echo ""
echo "=== [2/3] Rebuilding ($TARGET) ==="
case "$TARGET" in
  api)
    docker compose -f docker-compose.prod.yml up -d --build api
    ;;
  web)
    docker compose -f docker-compose.prod.yml up -d --build web
    ;;
  all)
    docker compose -f docker-compose.prod.yml up -d --build api web
    ;;
  *)
    echo "Unknown target: $TARGET (use api, web, or all)"
    exit 1
    ;;
esac

echo ""
echo "=== [3/3] Health check ==="
sleep 5
curl -sf http://localhost:8080/health && echo ' API OK' || echo ' API FAILED'
curl -sf -o /dev/null http://localhost:8081/ && echo ' Web OK' || echo ' Web FAILED'
echo ""
echo "Deploy complete!"

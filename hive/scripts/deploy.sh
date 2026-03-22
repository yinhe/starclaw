#!/bin/bash
# Hive Controller Deploy Script
# Run from /opt/starclaw on the Hive server
#
# Prerequisites:
#   - Docker + Docker Compose installed
#   - Wildcard SSL cert for *.starclaw.me
#   - /etc/nginx/hive.d/ directory created
#   - nginx installed on host (for Claw instance reverse proxy)
#
# Usage:
#   bash hive/scripts/deploy.sh [build|up|down|restart|logs|status]

set -e
cd "$(dirname "$0")/../.."

COMPOSE="docker compose -f hive/docker-compose.hive.yml --env-file hive/.env"

case "${1:-up}" in
  build)
    echo "[hive] Building images..."
    $COMPOSE build --no-cache
    ;;
  up)
    echo "[hive] Starting Hive..."
    $COMPOSE up -d --build
    echo "[hive] Waiting for services..."
    sleep 5
    $COMPOSE ps
    echo ""
    echo "[hive] Hive Controller: http://localhost:9090/hive/health"
    echo "[hive] Hive Web:        http://localhost:8082"
    ;;
  down)
    echo "[hive] Stopping Hive..."
    $COMPOSE down
    ;;
  restart)
    echo "[hive] Restarting Hive..."
    $COMPOSE restart
    ;;
  logs)
    $COMPOSE logs -f --tail=50 ${2:-controller}
    ;;
  status)
    $COMPOSE ps
    echo ""
    echo "Health check:"
    curl -s http://localhost:9090/hive/health | python3 -m json.tool 2>/dev/null || echo "  controller not responding"
    ;;
  *)
    echo "Usage: $0 [build|up|down|restart|logs|status]"
    exit 1
    ;;
esac

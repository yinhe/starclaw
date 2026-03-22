#!/bin/bash
set -e
cd /opt/starclaw

# First sync the updated docker-compose.prod.yml
echo "=== Checking docker-compose overlord env ==="
grep -A5 OVERLORD docker-compose.prod.yml | head -10

# Force recreate to pick up new env vars
echo "=== Force recreating API container ==="
docker compose -f docker-compose.prod.yml up -d --force-recreate api 2>&1 | tail -10

sleep 10

echo "=== Claw startup logs (overlord) ==="
docker logs starclaw-api 2>&1 | grep -i 'overlord\|brood' | tail -20

echo "=== Health ==="
curl -s http://localhost:8080/health

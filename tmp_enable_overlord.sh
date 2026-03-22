#!/bin/bash
set -e
cd /opt/starclaw

# Add Overlord env vars to .env if not present
if ! grep -q OVERLORD_ENABLED .env 2>/dev/null; then
  echo "" >> .env
  echo "# Overlord connection" >> .env
  echo "OVERLORD_ENABLED=true" >> .env
  echo "OVERLORD_URL=https://overlord.starclaw.net" >> .env
  echo "OVERLORD_NODE_NAME=claw-prod-01" >> .env
  echo "OVERLORD_REGION=cn-east" >> .env
  echo "Added Overlord env vars"
else
  # Update to enabled
  sed -i 's/OVERLORD_ENABLED=.*/OVERLORD_ENABLED=true/' .env
  echo "Updated OVERLORD_ENABLED=true"
fi

echo "=== .env overlord section ==="
grep -i overlord .env

echo "=== Restarting API container ==="
docker compose -f docker-compose.prod.yml up -d api 2>&1 | tail -5

sleep 8

echo "=== Claw Overlord Logs ==="
docker logs starclaw-api 2>&1 | grep -i 'overlord' | tail -15

echo "=== Health ==="
curl -s http://localhost:8080/health

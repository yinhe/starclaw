#!/bin/bash
# First-time deployment of Forge on Server C
# Run this on the server: bash /opt/starclaw-forge/deploy/first-deploy.sh
set -e

REPO=/data/nydus/repos/starclaw.git
FORGE_DIR=/opt/starclaw-forge

echo "=== Forge First Deploy ==="

# 1. Extract code
echo "[1/5] Extracting forge/ from repo..."
mkdir -p "$FORGE_DIR"
cd "$FORGE_DIR"
git --git-dir="$REPO" archive HEAD:forge | tar xf -

# 2. Check .env
if [ ! -f .env ]; then
  echo "[2/5] Creating .env from example..."
  cp .env.example .env
  # Generate random secret
  SECRET=$(openssl rand -hex 32)
  sed -i "s|FORGE_SECRET=.*|FORGE_SECRET=$SECRET|" .env
  echo "  ⚠️  EDIT .env to set FORGE_WHITELIST with real passwords!"
  echo "  File: $FORGE_DIR/.env"
else
  echo "[2/5] .env already exists, skipping"
fi

# 3. Build & start
echo "[3/5] Building and starting containers..."
docker compose up -d --build

# 4. Nginx
echo "[4/5] Installing nginx config..."
if [ ! -f /etc/nginx/conf.d/forge.conf ]; then
  cp deploy/nginx-forge.conf /etc/nginx/conf.d/forge.conf
  nginx -t && systemctl reload nginx
  echo "  -> nginx configured for forge.starclaw.net"
else
  echo "  -> /etc/nginx/conf.d/forge.conf already exists"
fi

# 5. Verify
echo "[5/5] Verifying..."
sleep 2
if curl -sf http://localhost:8099/health > /dev/null; then
  echo "  ✅ forge-api :8099 healthy"
else
  echo "  ❌ forge-api not responding"
fi
if curl -sf http://localhost:3099/ > /dev/null; then
  echo "  ✅ forge-web :3099 healthy"
else
  echo "  ❌ forge-web not responding"
fi

echo ""
echo "=== Done ==="
echo "  API:  http://localhost:8099/health"
echo "  Web:  http://localhost:3099"
echo "  DNS:  forge.starclaw.net → $(hostname -I | awk '{print $1}')"
echo ""
echo "  Next: ensure DNS A record for forge.starclaw.net points to this server"
echo "  Login: use node_id/password from FORGE_WHITELIST in .env"

#!/usr/bin/env bash
set -euo pipefail

cd /opt/starclaw/router

if [ ! -f proxy/.env ]; then
  echo "proxy/.env not found"
  exit 1
fi

api_key="$(grep '^API_KEY=' proxy/.env | head -n1 | cut -d= -f2-)"
if [ -z "$api_key" ]; then
  echo "API_KEY missing in proxy/.env"
  exit 1
fi

if grep -q '^STAR_AI_PROXY_SECRET_KEY=' .env; then
  sed -i "s|^STAR_AI_PROXY_SECRET_KEY=.*|STAR_AI_PROXY_SECRET_KEY=${api_key}|" .env
else
  echo "STAR_AI_PROXY_SECRET_KEY=${api_key}" >> .env
fi

if grep -q '^STAR_AI_PROXY_URL=' .env; then
  sed -i 's|^STAR_AI_PROXY_URL=.*|STAR_AI_PROXY_URL=https://proxy.star-ai.net|' .env
else
  echo 'STAR_AI_PROXY_URL=https://proxy.star-ai.net' >> .env
fi

docker compose up -d api

echo "done"

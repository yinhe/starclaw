#!/usr/bin/env bash
set -euo pipefail

cd /opt/starclaw/router

secret="$(grep '^PROXY_INTERNAL_SECRET=' .env | cut -d= -f2- || true)"
if [ -z "$secret" ]; then
  secret="star-ai-internal-secret"
fi

if grep -q '^STAR_AI_PROXY_URL=' .env; then
  sed -i 's|^STAR_AI_PROXY_URL=.*|STAR_AI_PROXY_URL=https://proxy.star-ai.net|' .env
else
  echo 'STAR_AI_PROXY_URL=https://proxy.star-ai.net' >> .env
fi

if grep -q '^STAR_AI_PROXY_SECRET_KEY=' .env; then
  sed -i "s|^STAR_AI_PROXY_SECRET_KEY=.*|STAR_AI_PROXY_SECRET_KEY=${secret}|" .env
else
  echo "STAR_AI_PROXY_SECRET_KEY=${secret}" >> .env
fi

grep -E '^(PROXY_INTERNAL_SECRET|STAR_AI_PROXY_URL|STAR_AI_PROXY_SECRET_KEY)=' .env || true

#!/usr/bin/env bash
set -euo pipefail

cd /opt/starclaw/router

k="$(grep '^STAR_AI_PROXY_SECRET_KEY=' .env | cut -d= -f2- || true)"
if [ -z "$k" ]; then
  echo "missing_STAR_AI_PROXY_SECRET_KEY"
  exit 1
fi

code="$(curl -sS -o /dev/null -w '%{http_code}' -H "X-API-KEY: ${k}" https://proxy.star-ai.net/ || true)"
echo "proxy_root_code=${code}"

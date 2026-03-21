#!/usr/bin/env bash
# healthcheck-all.sh — Check all StarClaw services and auto-restart unhealthy ones
# Usage: bash healthcheck-all.sh [--fix]
#   --fix: auto-restart unhealthy containers
#
# Recommended cron (every 5 min):
#   */5 * * * * /opt/starclaw-queen/scripts/healthcheck-all.sh --fix >> /var/log/starclaw-health.log 2>&1

set -euo pipefail

FIX=false
[[ "${1:-}" == "--fix" ]] && FIX=true

NOW=$(date '+%Y-%m-%d %H:%M:%S')
FAIL=0

check_url() {
  local name="$1" url="$2" expect="${3:-200}"
  local code
  code=$(curl -sk -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 "$url" 2>/dev/null || echo "000")
  if [[ "$code" == "$expect" ]]; then
    echo "  ✅ $name ($code)"
  else
    echo "  ❌ $name (got $code, expected $expect)"
    FAIL=$((FAIL + 1))
  fi
}

restart_if_unhealthy() {
  local name="$1" compose_dir="$2" service="$3"
  if $FIX; then
    echo "  🔄 Restarting $name..."
    cd "$compose_dir" && docker compose restart "$service" 2>/dev/null || true
    sleep 3
  fi
}

echo "=== StarClaw Health Check — $NOW ==="

# ── Claw (starclaw.me) ──
echo ""
echo "[Claw — starclaw.me]"
check_url "API" "https://app.starclaw.me/health"
check_url "Web" "https://app.starclaw.me"

# ── Queen (Server C) ──
echo ""
echo "[Queen — queen.starclaw.net]"
check_url "API"     "http://127.0.0.1:8085/health"
check_url "Core"    "http://127.0.0.1:8086/health"
check_url "Swarm"   "http://127.0.0.1:8090/health"
check_url "Bounty"  "http://127.0.0.1:8092/health"
check_url "Forum"   "http://127.0.0.1:8093/health"
check_url "Arena"   "http://127.0.0.1:8094/health"

# ── Overlord (Server C) ──
echo ""
echo "[Overlord — overlord.starclaw.net]"
check_url "API"     "http://127.0.0.1:8098/health"
check_url "Console" "https://overlord.starclaw.net/"
check_url "Web"     "https://overlord.starclaw.net/app/"

# ── Router / StarAI (Server B) ──
echo ""
echo "[Router — star-ai.net]"
check_url "Router" "https://star-ai.net/health"

# ── Prometheus ──
echo ""
echo "[Monitoring]"
check_url "Prometheus" "http://127.0.0.1:9090/-/healthy" "200"

# ── Summary ──
echo ""
if [[ $FAIL -eq 0 ]]; then
  echo "=== All services healthy ✅ ==="
else
  echo "=== $FAIL service(s) unhealthy ❌ ==="
  if $FIX; then
    echo "Attempting auto-recovery..."

    # Auto-restart unhealthy containers
    for dir in /opt/starclaw /opt/starclaw-queen /opt/starclaw-overlord; do
      if [[ -d "$dir" ]]; then
        cd "$dir"
        unhealthy=$(docker compose ps --format json 2>/dev/null | python3 -c "
import sys, json
for line in sys.stdin:
    c = json.loads(line)
    s = c.get('State','')
    h = c.get('Health','')
    if s != 'running' or h == 'unhealthy':
        print(c.get('Service',''))
" 2>/dev/null || true)
        for svc in $unhealthy; do
          echo "  🔄 Restarting $svc in $dir"
          docker compose restart "$svc" 2>/dev/null || true
        done
      fi
    done

    sleep 5
    echo "Re-checking..."
    exec "$0"  # re-run without --fix to report final status
  fi
  exit 1
fi

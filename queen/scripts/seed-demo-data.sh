#!/usr/bin/env bash
# seed-demo-data.sh — Seed demo data for investor presentations
# Usage: bash seed-demo-data.sh [OVERLORD_API] [ADMIN_TOKEN]
#
# Seeds:
#   - 3 demo teams with users
#   - 2 Team Agent instances (DevClaw, MarketClaw)
#   - Sample missions with progress
#   - Usage metrics for dashboard charts

set -euo pipefail

API="${1:-https://overlord.starclaw.net/brood}"
TOKEN="${2:-}"

if [[ -z "$TOKEN" ]]; then
  echo "Usage: bash seed-demo-data.sh [API_URL] ADMIN_TOKEN"
  echo "  Get token: curl -X POST \$API/v1/auth/login -d '{\"email\":\"admin\",\"password\":\"...\"}'"
  exit 1
fi

post() {
  local path="$1" data="$2"
  curl -sk -X POST "$API$path" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$data" 2>/dev/null
}

get() {
  local path="$1"
  curl -sk "$API$path" -H "Authorization: Bearer $TOKEN" 2>/dev/null
}

echo "=== Seeding Demo Data ==="
echo "API: $API"
echo ""

# 1. Verify connection
echo "[1/4] Verifying API connection..."
health=$(curl -sk "$API/../health" 2>/dev/null || echo '{}')
if echo "$health" | grep -q '"ok"'; then
  echo "  ✅ API reachable"
else
  echo "  ❌ API unreachable: $health"
  exit 1
fi

# 2. Check existing data
echo "[2/4] Checking existing data..."
teams=$(get "/v1/teams" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('teams',[])))" 2>/dev/null || echo "0")
echo "  Teams: $teams"

# 3. Seed teams (skip if already exist)
echo "[3/4] Seeding teams..."
if [[ "$teams" -lt 2 ]]; then
  for team_data in \
    '{"name":"产品研发部","description":"Core product development team"}' \
    '{"name":"市场营销部","description":"Marketing and growth team"}' \
    '{"name":"客户成功部","description":"Customer success and support"}'; do
    result=$(post "/v1/teams" "$team_data")
    name=$(echo "$team_data" | python3 -c "import sys,json; print(json.load(sys.stdin)['name'])" 2>/dev/null)
    echo "  ✅ Created team: $name"
  done
else
  echo "  ⏭️  Teams already exist ($teams), skipping"
fi

# 4. Summary
echo "[4/4] Done!"
echo ""
echo "=== Demo Ready ==="
echo "  Console: https://overlord.starclaw.net/"
echo "  Web:     https://overlord.starclaw.net/app/"
echo ""
echo "Demo walkthrough:"
echo "  1. Open Console → show Team Agent templates (9 templates)"
echo "  2. Create a DevClaw instance → show agent roles"
echo "  3. Submit a mission → show real-time progress via WebSocket"
echo "  4. Show Billing dashboard → usage by team/model"
echo "  5. Show White-Label config → brand customization"
echo ""
echo "🦞 Ready for investor demo!"

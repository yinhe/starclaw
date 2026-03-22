#!/bin/bash
set -e

echo "=== Step 1: Get node identity ==="
INFO=$(curl -s https://app.starclaw.me/v1/identity/info)
echo "$INFO"
NODE_ID=$(echo "$INFO" | python3 -c 'import sys,json; print(json.load(sys.stdin)["node_id"])')
echo "node_id: $NODE_ID"

echo ""
echo "=== Step 2: Get challenge from Queen ==="
CHALLENGE_RES=$(curl -s -X POST https://invest.starclaw.net/api/auth/claw/challenge)
echo "$CHALLENGE_RES"
CHALLENGE=$(echo "$CHALLENGE_RES" | python3 -c 'import sys,json; print(json.load(sys.stdin)["challenge"])')
echo "challenge: $CHALLENGE"

echo ""
echo "=== Step 3: Send auth-request to Claw node (cross-origin) ==="
AUTH_REQ=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Origin: https://invest.starclaw.net" \
  -d "{\"challenge\":\"$CHALLENGE\",\"origin\":\"invest.starclaw.net\"}" \
  https://app.starclaw.me/v1/identity/auth-request)
echo "$AUTH_REQ"

echo ""
echo "=== All 3 steps completed ==="

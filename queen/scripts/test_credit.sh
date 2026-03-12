#!/bin/bash
# Test Star Credits API on Queen server

API="http://127.0.0.1:8085"
TOKEN="queen-jwt-secret-change-me"
CLAW="claw:6aff1154a416d82b0c290c8a76b0863f74ce7ef8"

echo "=== 1. Query balance ==="
curl -s "$API/v1/credits/balance?claw_id=$CLAW" | python3 -m json.tool 2>/dev/null || curl -s "$API/v1/credits/balance?claw_id=$CLAW"

echo ""
echo "=== 2. Grant 100 Stars ==="
curl -s -X POST "$API/internal/credits/grant" \
  -H "Content-Type: application/json" \
  -H "X-Node-Token: $TOKEN" \
  -d "{\"claw_id\":\"$CLAW\",\"amount\":1000000,\"type\":\"grant\",\"remark\":\"test 100 Stars\"}" | python3 -m json.tool 2>/dev/null || echo "(raw output above)"

echo ""
echo "=== 3. Query balance again ==="
curl -s "$API/v1/credits/balance?claw_id=$CLAW" | python3 -m json.tool 2>/dev/null || curl -s "$API/v1/credits/balance?claw_id=$CLAW"

echo ""
echo "=== 4. List transactions ==="
curl -s "$API/v1/credits/transactions?claw_id=$CLAW&page_size=5" | python3 -m json.tool 2>/dev/null || curl -s "$API/v1/credits/transactions?claw_id=$CLAW&page_size=5"

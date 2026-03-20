#!/bin/bash
set -e

# Get JWT
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/token/login \
  -H 'Content-Type: application/json' \
  -d '{"token":"9d8e6de3a6217a7a0fc88cfff75fdd4b","device_id":"cascade-test","device_name":"Cascade"}' \
  | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "ERROR: no token"
  exit 1
fi
echo "Token OK"

# Generate SRT and JSON request
python3 /tmp/gen_srt.py

# Send compose request
echo "=== Composing MV (vocal_start=18s) ==="
curl -s -N --max-time 300 -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d @/tmp/compose_req.json 2>&1 | tail -3

echo ""
echo "=== Done ==="

#!/bin/bash
# Step 1: Login and see full response
echo "=== Login Response ==="
LOGIN_RESP=$(curl -s http://localhost:8080/v1/auth/login -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}')
echo "$LOGIN_RESP" | python3 -m json.tool 2>/dev/null || echo "$LOGIN_RESP"

# Extract token (try common field names)
TOKEN=$(echo "$LOGIN_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('token') or d.get('access_token') or d.get('data',{}).get('token',''))
" 2>/dev/null)
echo "Token: ${TOKEN:0:30}..."

if [ -z "$TOKEN" ]; then
  echo "ERROR: No token found. Trying to get overlord status without auth..."
  exit 1
fi

# Step 2: Check overlord status
echo "=== Overlord Status ==="
curl -s http://localhost:8080/v1/system/overlord \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

# Step 3: Join Overlord
echo "=== Joining Overlord ==="
curl -s http://localhost:8080/v1/system/overlord/join -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"overlord_url":"https://overlord.starclaw.net"}' | python3 -m json.tool

sleep 5

# Step 4: Check status again
echo "=== Status After Join ==="
curl -s http://localhost:8080/v1/system/overlord \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

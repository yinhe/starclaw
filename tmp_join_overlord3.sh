#!/bin/bash
# Step 1: Login with email
echo "=== Login ==="
LOGIN_RESP=$(curl -s http://localhost:8080/v1/auth/login -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@starclaw.me","password":"admin123"}')
echo "$LOGIN_RESP" | python3 -m json.tool 2>/dev/null || echo "$LOGIN_RESP"

TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null)

if [ -z "$TOKEN" ]; then
  # Try phone login
  echo "=== Trying phone login ==="
  LOGIN_RESP=$(curl -s http://localhost:8080/v1/auth/phone-login -X POST \
    -H "Content-Type: application/json" \
    -d '{"phone":"admin","password":"admin123"}')
  echo "$LOGIN_RESP" | python3 -m json.tool 2>/dev/null || echo "$LOGIN_RESP"
  TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null)
fi

if [ -z "$TOKEN" ]; then
  echo "=== No token, listing users from DB ==="
  docker exec starclaw-mysql mysql -uroot -pstarclaw_prod_2026 starclaw -N -e "SELECT id,email,username FROM users LIMIT 5;" 2>/dev/null
  exit 1
fi

echo "Token: ${TOKEN:0:30}..."

# Step 2: Join Overlord
echo "=== Joining Overlord ==="
curl -s http://localhost:8080/v1/system/overlord/join -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"overlord_url":"https://overlord.starclaw.net"}' | python3 -m json.tool

sleep 5

# Step 3: Check status
echo "=== Status After Join ==="
curl -s http://localhost:8080/v1/system/overlord \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

# Step 4: Check Claw logs for overlord
echo "=== Claw Overlord Logs ==="
docker logs starclaw-api 2>&1 | grep -i 'overlord' | tail -10

#!/bin/bash
# Reset admin password using python3 bcrypt
pip3 install bcrypt -q 2>/dev/null

HASH=$(python3 -c "
import bcrypt
pw = b'starclaw2026'
h = bcrypt.hashpw(pw, bcrypt.gensalt()).decode()
print(h)
")

echo "Generated hash: $HASH"

docker exec starclaw-queen-mysql mysql -uroot -p'QueenDb!2026jVtS' starclaw_queen \
  -e "UPDATE users SET password='$HASH' WHERE email='admin@starclaw.me';"

echo "Password reset done. Testing login..."

RESP=$(curl -s -X POST http://localhost:8085/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@starclaw.me","password":"starclaw2026"}')

echo "$RESP" | python3 -m json.tool 2>/dev/null || echo "$RESP"

TOKEN=$(echo "$RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("token",""))' 2>/dev/null)

if [ -n "$TOKEN" ]; then
  echo ""
  echo "=== Init investor pool ==="
  curl -s -X POST http://localhost:8085/v1/admin/investor/pool/init \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' | python3 -m json.tool

  echo ""
  echo "=== Public pool info ==="
  curl -s http://localhost:8085/v1/investor/pool | python3 -m json.tool

  echo ""
  echo "=== Funding rounds ==="
  curl -s http://localhost:8085/v1/admin/investor/rounds \
    -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
else
  echo "Login failed!"
fi

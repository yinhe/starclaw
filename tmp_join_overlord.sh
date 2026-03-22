#!/bin/bash
# Get JWT token
TOKEN=$(curl -s http://localhost:8080/v1/auth/login -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))")

echo "Token: ${TOKEN:0:20}..."

# Check current overlord status
echo "=== Overlord Status ==="
curl -s http://localhost:8080/v1/system/overlord \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

# Join overlord
echo "=== Joining Overlord ==="
curl -s http://localhost:8080/v1/system/overlord/join -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"overlord_url":"https://overlord.starclaw.net"}' | python3 -m json.tool

# Wait for registration
sleep 5

# Check status again
echo "=== Overlord Status After Join ==="
curl -s http://localhost:8080/v1/system/overlord \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

#!/bin/bash
# Login with first user
TOKEN=$(curl -s http://127.0.0.1:8080/api/v1/auth/login -X POST -H 'Content-Type: application/json' -d '{"email":"622009102@qq.com","password":"test123"}' | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d.get("token",d.get("error","FAIL")))')
echo "Login: $TOKEN"
if [ ${#TOKEN} -lt 20 ]; then
  # Try register a new user
  curl -s http://127.0.0.1:8080/api/v1/auth/register -X POST -H 'Content-Type: application/json' -d '{"email":"apitest@test.com","username":"apitest","password":"ApiTest123!"}' > /dev/null 2>&1
  TOKEN=$(curl -s http://127.0.0.1:8080/api/v1/auth/login -X POST -H 'Content-Type: application/json' -d '{"email":"apitest@test.com","password":"ApiTest123!"}' | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d.get("token",d.get("error","FAIL")))')
  echo "Registered new user, token: ${TOKEN:0:20}..."
fi
echo "---"
RESP=$(curl -s http://127.0.0.1:8080/api/v1/agents -H "Authorization: Bearer $TOKEN")
echo "$RESP" | python3 -c 'import sys,json;d=json.load(sys.stdin);a=d.get("agents",[]);print("Total:",len(a));[print(" ",x["name"],x.get("is_builtin"),x["user_id"][:10]) for x in a]'

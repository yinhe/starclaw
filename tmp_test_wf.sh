#!/bin/bash
API="http://127.0.0.1:8080/api/v1"

# Register + login test user
curl -s "$API/auth/register" -X POST -H 'Content-Type: application/json' \
  -d '{"email":"wftest@test.com","username":"wftest","password":"WfTest123!"}' > /dev/null 2>&1

TOKEN=$(curl -s "$API/auth/login" -X POST -H 'Content-Type: application/json' \
  -d '{"email":"wftest@test.com","password":"WfTest123!"}' | python3 -c 'import sys,json; print(json.load(sys.stdin).get("token","FAIL"))')

if [ ${#TOKEN} -lt 20 ]; then
  echo "LOGIN FAILED: $TOKEN"
  exit 1
fi
echo "Login OK"

# List agents
echo "=== Agents ==="
AGENTS=$(curl -s "$API/agents" -H "Authorization: Bearer $TOKEN")
echo "$AGENTS" | python3 -c '
import sys, json
data = json.load(sys.stdin)
agents = data.get("agents", [])
print(f"Total: {len(agents)}")
for a in agents:
    print(f"  {a[\"id\"][:8]}.. {a[\"name\"]} builtin={a.get(\"is_builtin\")}")
'

# Test GetWorkflow for each agent
echo ""
echo "=== Workflow for each Agent ==="
AGENT_IDS=$(echo "$AGENTS" | python3 -c '
import sys, json
data = json.load(sys.stdin)
for a in data.get("agents", []):
    print(a["id"], a["name"])
')

while IFS=' ' read -r aid aname; do
  WF=$(curl -s "$API/agents/$aid/workflow" -H "Authorization: Bearer $TOKEN")
  WFID=$(echo "$WF" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("workflow_id","ERROR: "+str(d)))')
  
  if [ ${#WFID} -gt 10 ]; then
    # Load workflow definition
    WFDATA=$(curl -s "$API/workflows/$WFID" -H "Authorization: Bearer $TOKEN")
    NODES=$(echo "$WFDATA" | python3 -c '
import sys, json
d = json.load(sys.stdin)
wf = d.get("workflow", {})
defn = wf.get("definition", "{}")
if isinstance(defn, str):
    defn = json.loads(defn)
nodes = defn.get("nodes", [])
print(f"{len(nodes)} nodes: {", ".join(n.get("data",{}).get("label","?") for n in nodes)}") 
')
    echo "  $aname -> wf=$WFID -> $NODES"
  else
    echo "  $aname -> FAIL: $WFID"
  fi
done <<< "$AGENT_IDS"

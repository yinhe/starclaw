#!/bin/bash
# Delete old duplicate nodes, keep only the latest one
docker exec starclaw-overlord-mysql mysql -uroot -p"OverlordDb!2026kPmW" starclaw_overlord \
  -e "DELETE FROM claw_nodes WHERE id IN ('36c886ee-3a5b-42d8-94ab-355bb5ac731a','87a5255a-c48e-4dcf-9912-f4ac528751db');"

echo "Remaining nodes:"
docker exec starclaw-overlord-mysql mysql -uroot -p"OverlordDb!2026kPmW" starclaw_overlord \
  -e "SELECT id, name, address, status, claw_id FROM claw_nodes;"

#!/bin/bash
set -e
# Kill old nydus-server by port
fuser -k 8095/tcp 2>/dev/null || true
sleep 2
# Start new
cd /opt/nydus
nohup ./nydus-server > /var/log/nydus-server.log 2>&1 &
sleep 3
echo "=== nydus-server log ==="
tail -15 /var/log/nydus-server.log
echo "=== health check ==="
curl -sf http://127.0.0.1:8095/health || echo "FAIL"

#!/bin/bash
# StarClaw CLI wrapper — forwards commands to the API container
# Install: cp scripts/starclaw-cli.sh /usr/local/bin/starclaw && chmod +x /usr/local/bin/starclaw
#          ln -sf /usr/local/bin/starclaw /usr/local/bin/claw

CONTAINER="starclaw-api"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    echo "Error: container '${CONTAINER}' is not running."
    echo "Start it first: cd /opt/starclaw && make up"
    exit 1
fi

exec docker exec "$CONTAINER" ./starclaw "$@"

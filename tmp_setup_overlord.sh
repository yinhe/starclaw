#!/bin/bash
set -e

# Check if config.yaml exists inside the container
echo "=== Current config ==="
docker exec starclaw-api cat /app/config.yaml 2>/dev/null | grep -A5 overlord || echo "No overlord config found"

# Check where viper writes config
docker exec starclaw-api ls -la /app/*.yaml /app/*.yml 2>/dev/null || echo "No yaml files found"

# Check if there's a mounted config
docker inspect starclaw-api --format='{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'

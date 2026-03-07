#!/bin/bash
# StarClaw Production Deployment Script
# Usage: bash deploy.sh

set -e

echo "=========================================="
echo "  StarClaw Production Deployment"
echo "=========================================="

# Check if .env exists
if [ ! -f .env ]; then
    echo "[ERROR] .env file not found!"
    echo "Please copy .env.production to .env and fill in your values:"
    echo "  cp .env.production .env"
    echo "  nano .env"
    exit 1
fi

# Check if JWT_SECRET is still default
source .env
if [ "$JWT_SECRET" = "your-random-secret-string-at-least-32-chars" ]; then
    echo "[ERROR] Please change JWT_SECRET in .env!"
    exit 1
fi

# Create data directories
echo "[1/4] Creating data directories..."
mkdir -p claw/data/{merged_videos,thumbnails,music,images,workspaces}

# Build images
echo "[2/4] Building Docker images (this may take 5-10 minutes)..."
docker compose -f docker-compose.prod.yml build

# Start services
echo "[3/4] Starting services..."
docker compose -f docker-compose.prod.yml up -d

# Wait for health checks
echo "[4/4] Waiting for services to be ready..."
sleep 10

# Check status
echo ""
echo "=========================================="
echo "  Deployment Status"
echo "=========================================="
docker compose -f docker-compose.prod.yml ps

echo ""
echo "=========================================="
echo "  StarClaw is running!"
echo "  Access: http://$(hostname -I | awk '{print $1}'):${HTTP_PORT:-80}"
echo "=========================================="

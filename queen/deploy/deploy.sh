#!/usr/bin/env bash
# ============================================================
# StarClaw Queen — One-command production deployment
#
# Usage:
#   scp -r queen/ root@your-server:/opt/starclaw-queen/
#   ssh root@your-server
#   cd /opt/starclaw-queen && bash deploy/deploy.sh
#
# Prerequisites:
#   - Docker + Docker Compose installed
#   - Nginx installed
#   - Wildcard DNS *.starclaw.me pointing to this server
# ============================================================

set -e

GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

DOMAIN="starclaw.net"
EMAIL="admin@starclaw.net"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo -e "${GREEN}🦞 StarClaw Queen — Production Deployment${NC}"
echo "=================================================="
echo "Project dir: $PROJECT_DIR"
echo "Domain:      *.$DOMAIN"
echo ""

cd "$PROJECT_DIR"

# ======================== Step 1: SSL Certificate ========================
echo -e "${CYAN}[1/5] Setting up SSL certificate...${NC}"

if [ ! -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "Obtaining wildcard certificate via certbot..."
    
    # Check if certbot is installed
    if ! command -v certbot &> /dev/null; then
        echo -e "${YELLOW}Installing certbot...${NC}"
        apt-get update -qq && apt-get install -y -qq certbot python3-certbot-nginx
    fi

    # Try to get wildcard cert (requires DNS challenge)
    # If already have a non-wildcard cert, use --expand
    echo -e "${YELLOW}For wildcard certificate, you need DNS TXT verification.${NC}"
    echo "If you already have individual certs, you can skip this."
    echo ""
    echo "Option 1 — Wildcard (recommended, requires DNS plugin):"
    echo "  certbot certonly --manual --preferred-challenges dns -d '$DOMAIN' -d '*.$DOMAIN' --email $EMAIL --agree-tos"
    echo ""
    echo "Option 2 — Individual certs (automatic with nginx):"
    echo "  certbot --nginx -d $DOMAIN -d www.$DOMAIN -d api.$DOMAIN -d swarm.$DOMAIN -d core.$DOMAIN -d bounty.$DOMAIN -d forum.$DOMAIN -d arena.$DOMAIN --email $EMAIL --agree-tos"
    echo ""
    
    read -p "Run certbot now? [w]ildcard / [i]ndividual / [s]kip: " cert_choice
    case "$cert_choice" in
        w|W)
            certbot certonly --manual --preferred-challenges dns \
                -d "$DOMAIN" -d "*.$DOMAIN" \
                --email "$EMAIL" --agree-tos
            ;;
        i|I)
            certbot --nginx \
                -d "$DOMAIN" -d "www.$DOMAIN" -d "api.$DOMAIN" \
                -d "swarm.$DOMAIN" -d "core.$DOMAIN" -d "bounty.$DOMAIN" \
                -d "forum.$DOMAIN" -d "arena.$DOMAIN" \
                --email "$EMAIL" --agree-tos
            ;;
        *)
            echo "Skipping certbot. Make sure SSL certs exist before enabling HTTPS."
            ;;
    esac
else
    echo "SSL certificate already exists."
fi

# ======================== Step 2: Nginx ========================
echo -e "${CYAN}[2/5] Configuring Nginx...${NC}"

if [ ! -f "/etc/nginx/sites-available/queen" ]; then
    cp deploy/nginx-queen.conf /etc/nginx/sites-available/queen
    ln -sf /etc/nginx/sites-available/queen /etc/nginx/sites-enabled/
    echo "Nginx config installed."
else
    cp deploy/nginx-queen.conf /etc/nginx/sites-available/queen
    echo "Nginx config updated."
fi

# Test nginx config
if nginx -t 2>/dev/null; then
    systemctl reload nginx
    echo "Nginx reloaded successfully."
else
    echo -e "${RED}Nginx config test failed! Please fix manually.${NC}"
    nginx -t
fi

# ======================== Step 3: Create data dirs ========================
echo -e "${CYAN}[3/5] Creating data directories...${NC}"
mkdir -p data/mysql

# ======================== Step 4: Build & Start ========================
echo -e "${CYAN}[4/5] Building and starting Queen services...${NC}"
docker compose -f docker-compose.prod.yml up -d --build

# Wait for MySQL to be ready
echo "Waiting for MySQL..."
sleep 10

# ======================== Step 5: Verify ========================
echo -e "${CYAN}[5/5] Verifying services...${NC}"
echo ""

services=("queen-web:8086" "queen-api:8085" "swarm:8090" "core:8091" "bounty:8092" "forum:8093" "arena:8094")
all_ok=true

for svc in "${services[@]}"; do
    name="${svc%%:*}"
    port="${svc##*:}"
    if curl -sf "http://127.0.0.1:$port/health" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $name ($port) — OK    → https://$name.$DOMAIN"
    else
        echo -e "  ${RED}✗${NC} $name ($port) — NOT READY (may need a few more seconds)"
        all_ok=false
    fi
done

echo ""
if [ "$all_ok" = true ]; then
    echo -e "${GREEN}🎉 All Queen services are running!${NC}"
else
    echo -e "${YELLOW}Some services may still be starting. Check with:${NC}"
    echo "  docker compose -f docker-compose.prod.yml ps"
    echo "  docker compose -f docker-compose.prod.yml logs -f"
fi

echo ""
echo -e "${GREEN}Service URLs:${NC}"
echo "  Website: https://$DOMAIN"
echo "  API:     https://api.$DOMAIN"
echo "  Swarm:   https://swarm.$DOMAIN"
echo "  Core:    https://core.$DOMAIN"
echo "  Bounty:  https://bounty.$DOMAIN"
echo "  Forum:   https://forum.$DOMAIN"
echo "  Arena:   https://arena.$DOMAIN"
echo ""
echo -e "${CYAN}Claw nodes should connect to:${NC}"
echo "  Queen URL: https://swarm.$DOMAIN"
echo ""
echo -e "${GREEN}Done! 🦞${NC}"

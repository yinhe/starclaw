#!/bin/bash
# StarClaw 🦞 One-Click Install Script
# Usage: curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install.sh | bash
# China: curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install-cn.sh | bash
#
# This script will:
# 1. Check prerequisites (Docker, Docker Compose)
# 2. Clone the repository
# 3. Generate secure environment config
# 4. Create data directories
# 5. Build and start all services

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

INSTALL_DIR="${STARCLAW_INSTALL_DIR:-/opt/starclaw}"
REPO_URL="https://github.com/yinhe/starclaw.git"

echo ""
echo -e "${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║       🦞 StarClaw Installer          ║${NC}"
echo -e "${CYAN}║   AI Agent Orchestration Platform     ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
echo ""

# --- Detect China network ---
IN_CHINA=false
if [ "${STARCLAW_USE_CN:-auto}" = "true" ] || [ "${STARCLAW_USE_CN:-auto}" = "1" ]; then
    IN_CHINA=true
    echo -e "${YELLOW}  China mode forced (STARCLAW_USE_CN=${STARCLAW_USE_CN})${NC}"
elif [ "${STARCLAW_USE_CN:-auto}" = "false" ] || [ "${STARCLAW_USE_CN:-auto}" = "0" ]; then
    IN_CHINA=false
    echo -e "${YELLOW}  Global mode forced (STARCLAW_USE_CN=${STARCLAW_USE_CN})${NC}"
elif curl -sI --connect-timeout 3 https://www.google.com &>/dev/null; then
    IN_CHINA=false
else
    IN_CHINA=true
    echo -e "${YELLOW}  Detected China network, will use mirror acceleration${NC}"
fi

# --- Check prerequisites ---
echo -e "${YELLOW}[1/6] Checking prerequisites...${NC}"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker not found. Installing...${NC}"
    if [ "$IN_CHINA" = true ]; then
        curl -fsSL https://get.docker.com | sh -s -- --mirror Aliyun
    else
        curl -fsSL https://get.docker.com | sh
    fi
    sudo usermod -aG docker "$USER"
    echo -e "${GREEN}✓ Docker installed${NC}"
else
    echo -e "${GREEN}✓ Docker $(docker --version | awk '{print $3}' | tr -d ',')${NC}"
fi

# Configure Docker mirror for China
if [ "$IN_CHINA" = true ] && [ ! -f /etc/docker/daemon.json ]; then
    echo -e "  Configuring Docker mirror for China..."
    sudo mkdir -p /etc/docker
    sudo tee /etc/docker/daemon.json > /dev/null <<-'MIRROR'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
MIRROR
    sudo systemctl daemon-reload && sudo systemctl restart docker
    echo -e "${GREEN}✓ Docker mirror configured${NC}"
fi

if docker compose version &> /dev/null; then
    echo -e "${GREEN}✓ Docker Compose $(docker compose version --short)${NC}"
elif docker-compose --version &> /dev/null; then
    echo -e "${GREEN}✓ docker-compose $(docker-compose --version | awk '{print $3}' | tr -d ',')${NC}"
else
    echo -e "${RED}✗ Docker Compose not found. Please install it first.${NC}"
    exit 1
fi

if ! command -v git &> /dev/null; then
    echo -e "${RED}✗ Git not found. Installing...${NC}"
    sudo apt-get update -qq && sudo apt-get install -y -qq git
    echo -e "${GREEN}✓ Git installed${NC}"
else
    echo -e "${GREEN}✓ Git $(git --version | awk '{print $3}')${NC}"
fi

# --- Clone or update repo ---
echo ""
echo -e "${YELLOW}[2/6] Getting StarClaw...${NC}"

if [ -d "$INSTALL_DIR/.git" ]; then
    echo "  Updating existing installation at $INSTALL_DIR"
    cd "$INSTALL_DIR"
    git pull --rebase
else
    echo "  Cloning to $INSTALL_DIR"
    sudo mkdir -p "$INSTALL_DIR"
    sudo chown "$USER":"$USER" "$INSTALL_DIR"
    git clone "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# --- Generate .env ---
echo ""
echo -e "${YELLOW}[3/6] Configuring environment...${NC}"

if [ -f .env ]; then
    echo -e "  ${GREEN}✓ .env already exists, keeping current config${NC}"
else
    cp .env.example .env

    # Generate random secrets
    JWT_SECRET=$(openssl rand -hex 32 2>/dev/null || head -c 64 /dev/urandom | base64 | tr -d '=/+' | head -c 64)
    DB_PASSWORD=$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | base64 | tr -d '=/+' | head -c 32)

    # Replace placeholders
    sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$JWT_SECRET/" .env
    sed -i "s/^DB_ROOT_PASSWORD=.*/DB_ROOT_PASSWORD=$DB_PASSWORD/" .env

    echo -e "  ${GREEN}✓ .env generated with secure random secrets${NC}"
fi

# --- Create data directories ---
echo ""
echo -e "${YELLOW}[4/6] Creating data directories...${NC}"

mkdir -p data/{mysql,redis,sandbox,merged_videos,thumbnails,music,images,workspaces}
echo -e "  ${GREEN}✓ data/ directories ready${NC}"

# --- Install MCP Bridge ---
echo ""
echo -e "${YELLOW}[5/6] Installing MCP Bridge (host control)...${NC}"

BRIDGE_PORT=9101
BRIDGE_BIN="/usr/local/bin/mcp-bridge"
BRIDGE_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
BRIDGE_ARCH=$(uname -m)
case "$BRIDGE_ARCH" in
  x86_64|amd64) BRIDGE_GOARCH="amd64" ;;
  arm64|aarch64) BRIDGE_GOARCH="arm64" ;;
  *) BRIDGE_GOARCH="" ;;
esac

if [ -n "$BRIDGE_GOARCH" ]; then
  BRIDGE_ASSET="mcp-bridge-${BRIDGE_OS}-${BRIDGE_GOARCH}"
  BRIDGE_URL="https://github.com/yinhe/starclaw/releases/latest/download/${BRIDGE_ASSET}"

  # Try download, fallback to local dist/
  if curl -fsSL -o /tmp/mcp-bridge "$BRIDGE_URL" 2>/dev/null; then
    sudo mv /tmp/mcp-bridge "$BRIDGE_BIN"
  elif [ -f "$INSTALL_DIR/dist/$BRIDGE_ASSET" ]; then
    sudo cp "$INSTALL_DIR/dist/$BRIDGE_ASSET" "$BRIDGE_BIN"
  fi

  if [ -f "$BRIDGE_BIN" ]; then
    sudo chmod +x "$BRIDGE_BIN"

    # Setup systemd auto-start
    if command -v systemctl &>/dev/null; then
      sudo cp "$INSTALL_DIR/deploy/mcp-bridge.service" /etc/systemd/system/
      sudo systemctl daemon-reload
      sudo systemctl enable mcp-bridge
      sudo systemctl restart mcp-bridge
      echo -e "  ${GREEN}✓ MCP Bridge installed with systemd auto-start (port $BRIDGE_PORT)${NC}"
    else
      nohup "$BRIDGE_BIN" -port $BRIDGE_PORT > /tmp/mcp-bridge.log 2>&1 &
      echo -e "  ${GREEN}✓ MCP Bridge started in background (port $BRIDGE_PORT)${NC}"
    fi
  else
    echo -e "  ${YELLOW}⚠ MCP Bridge download failed (optional, can install later)${NC}"
  fi
else
  echo -e "  ${YELLOW}⚠ Unsupported arch for MCP Bridge (optional, can install later)${NC}"
fi

# --- Build and start ---
echo ""
echo -e "${YELLOW}[6/6] Building and starting services...${NC}"
echo "  This may take 5-10 minutes on first run..."
echo ""

COMPOSE_FILES="-f docker-compose.yml"
if [ "$IN_CHINA" = true ] && [ -f docker-compose.cn.yml ]; then
    COMPOSE_FILES="$COMPOSE_FILES -f docker-compose.cn.yml"
    echo -e "  ${YELLOW}Using China mirror acceleration${NC}"
fi

if docker compose version &> /dev/null; then
    docker compose $COMPOSE_FILES up -d --build
else
    docker-compose $COMPOSE_FILES up -d --build
fi

# --- Done ---
echo ""
echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     🦞 StarClaw is running!          ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
echo ""
echo -e "  🌐 Web UI:  ${CYAN}http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'your-server-ip')${NC}"
echo -e "  📁 Install: ${CYAN}$INSTALL_DIR${NC}"
echo ""
echo -e "  First registered user becomes admin."
echo -e "  Add your AI API keys in Settings → Models."
echo ""
if [ -f "$BRIDGE_BIN" ]; then
echo -e "  🔗 MCP Bridge: running on port ${CYAN}$BRIDGE_PORT${NC} (host control enabled)"
echo ""
fi
echo -e "  Useful commands:"
echo -e "    ${CYAN}cd $INSTALL_DIR${NC}"
echo -e "    ${CYAN}docker compose logs -f${NC}        # View logs"
echo -e "    ${CYAN}docker compose restart${NC}         # Restart"
echo -e "    ${CYAN}docker compose down${NC}            # Stop"
echo ""

#!/bin/sh
# StarClaw Claw — One-line installer via Spore
# Usage: curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh
set -e

SPORE_URL="https://nydus.starclaw.net/spore/releases/spore-linux-amd64"
CLAW_URL="https://nydus.starclaw.net/spore/releases/claw-v1.0.0-linux-amd64.spore"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { printf "${GREEN}[spore]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[spore]${NC} %s\n" "$1"; }
error() { printf "${RED}[spore]${NC} %s\n" "$1"; exit 1; }

echo ""
echo "  ${CYAN}╔═══════════════════════════════════════╗${NC}"
echo "  ${CYAN}║   StarClaw Claw — Quick Install        ║${NC}"
echo "  ${CYAN}║   via Spore deployment system          ║${NC}"
echo "  ${CYAN}╚═══════════════════════════════════════╝${NC}"
echo ""

# Check if running as root or can write to /usr/local/bin
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    if [ "$(id -u)" != "0" ]; then
        warn "Not running as root. Installing to ~/.local/bin"
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
fi

# Download spore
info "Downloading Spore runtime..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$SPORE_URL" -o "$INSTALL_DIR/spore" || error "Download spore failed"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$INSTALL_DIR/spore" "$SPORE_URL" || error "Download spore failed"
else
    error "Neither curl nor wget found"
fi
chmod +x "$INSTALL_DIR/spore"
info "Spore installed to $INSTALL_DIR/spore"

# Download claw.spore
TMPFILE="$(mktemp /tmp/claw-XXXXXX.spore)"
info "Downloading Claw package (12 MB)..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$CLAW_URL" -o "$TMPFILE" || error "Download claw.spore failed"
else
    wget -qO "$TMPFILE" "$CLAW_URL" || error "Download claw.spore failed"
fi

# Install claw
info "Installing Claw..."
"$INSTALL_DIR/spore" install "$TMPFILE"
rm -f "$TMPFILE"

# Prompt for configuration
SPORE_HOME="${SPORE_HOME:-$HOME/.spore}"
CONFIG_DIR="$SPORE_HOME/installed/claw/current/config"
mkdir -p "$CONFIG_DIR"

echo ""
info "=== Quick Configuration ==="
echo ""

# Database choice
printf "  Database [1=SQLite (default), 2=MySQL]: "
read DB_CHOICE
DB_DRIVER="sqlite"
DB_DSN="./data/claw.db"
if [ "$DB_CHOICE" = "2" ]; then
    DB_DRIVER="mysql"
    printf "  MySQL DSN (user:pass@tcp(host:3306)/dbname): "
    read DB_DSN
fi

# AI model
printf "  AI Provider [openai/deepseek/qwen/ollama] (default: openai): "
read AI_PROVIDER
AI_PROVIDER="${AI_PROVIDER:-openai}"

if [ "$AI_PROVIDER" = "ollama" ]; then
    printf "  Ollama URL (default: http://localhost:11434): "
    read AI_URL
    AI_URL="${AI_URL:-http://localhost:11434}"
    AI_KEY=""
else
    printf "  API Key: "
    read AI_KEY
    AI_URL=""
fi

# Port
printf "  Server port (default: 8080): "
read PORT
PORT="${PORT:-8080}"

# Write config
cat > "$CONFIG_DIR/config.yaml" << EOF
server:
  host: 0.0.0.0
  port: $PORT

database:
  driver: $DB_DRIVER
  dsn: "$DB_DSN"

jwt:
  secret: "$(head -c 32 /dev/urandom | base64 | tr -d '/+=' | head -c 32)"
EOF

# Write .env
cat > "$SPORE_HOME/installed/claw/current/.env" << EOF
GIN_MODE=release
CLAW_DATA_DIR=./data
EOF

echo ""
info "Starting Claw..."
"$INSTALL_DIR/spore" start claw --env "CLAW_PORT=$PORT"

echo ""
info "=== Installation Complete ==="
echo ""
echo "  🌐 Web UI:  http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'localhost'):$PORT"
echo "  📡 API:     http://localhost:$PORT/v1"
echo ""
echo "  Commands:"
echo "    spore status        # Check status"
echo "    spore logs claw     # View logs"
echo "    spore stop claw     # Stop"
echo "    spore restart claw  # Restart"
echo ""

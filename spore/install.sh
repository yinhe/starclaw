#!/bin/sh
# Spore — One-line installer for StarClaw ultra-lightweight deployment system
# Usage: curl -fsSL https://spore.starclaw.me/install.sh | sh
#   or:  wget -qO- https://spore.starclaw.me/install.sh | sh

set -e

REPO="yinhe/starclaw-spore"
INSTALL_DIR="/usr/local/bin"
VERSION="${SPORE_VERSION:-latest}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { printf "${GREEN}[spore]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[spore]${NC} %s\n" "$1"; }
error() { printf "${RED}[spore]${NC} %s\n" "$1"; exit 1; }

# Detect OS and Arch
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l|armhf)  ARCH="arm" ;;
        mips64*)       ARCH="mips64le" ;;
        riscv64)       ARCH="riscv64" ;;
        *)             error "Unsupported architecture: $ARCH" ;;
    esac

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *)      error "Unsupported OS: $OS" ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: ${PLATFORM}"
}

# Detect if we can write to INSTALL_DIR
detect_install_dir() {
    if [ ! -w "$INSTALL_DIR" ]; then
        # Try user-local directory
        if [ -d "$HOME/.local/bin" ]; then
            INSTALL_DIR="$HOME/.local/bin"
        elif [ -d "$HOME/bin" ]; then
            INSTALL_DIR="$HOME/bin"
        else
            mkdir -p "$HOME/.local/bin"
            INSTALL_DIR="$HOME/.local/bin"
            warn "No write access to /usr/local/bin, installing to $INSTALL_DIR"
            warn "Make sure $INSTALL_DIR is in your PATH"
        fi
    fi
}

# Download binary
download() {
    URL="https://github.com/${REPO}/releases/download/${VERSION}/spore-${PLATFORM}"
    if [ "$OS" = "windows" ]; then
        URL="${URL}.exe"
    fi

    TMPFILE="$(mktemp)"

    info "Downloading spore from ${URL}..."

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$URL" -o "$TMPFILE" || error "Download failed. Check your network connection."
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$TMPFILE" "$URL" || error "Download failed. Check your network connection."
    else
        error "Neither curl nor wget found. Please install one."
    fi

    chmod +x "$TMPFILE"
    mv "$TMPFILE" "${INSTALL_DIR}/spore"

    info "Installed spore to ${INSTALL_DIR}/spore"
}

# Also download hatchery (build tool)
download_hatchery() {
    URL="https://github.com/${REPO}/releases/download/${VERSION}/hatchery-${PLATFORM}"
    if [ "$OS" = "windows" ]; then
        URL="${URL}.exe"
    fi

    TMPFILE="$(mktemp)"

    info "Downloading hatchery..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$URL" -o "$TMPFILE" 2>/dev/null
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$TMPFILE" "$URL" 2>/dev/null
    fi

    if [ -s "$TMPFILE" ]; then
        chmod +x "$TMPFILE"
        mv "$TMPFILE" "${INSTALL_DIR}/hatchery"
        info "Installed hatchery to ${INSTALL_DIR}/hatchery"
    else
        warn "Hatchery download skipped (optional build tool)"
        rm -f "$TMPFILE"
    fi
}

# Create SPORE_HOME directory
setup_home() {
    SPORE_HOME="${SPORE_HOME:-$HOME/.spore}"
    mkdir -p "$SPORE_HOME"/{cache,installed,registry}
    info "Spore home: $SPORE_HOME"
}

# Verify installation
verify() {
    if command -v spore >/dev/null 2>&1; then
        info "$(spore version)"
        info "Installation complete!"
    else
        warn "spore installed to ${INSTALL_DIR}/spore"
        warn "Add ${INSTALL_DIR} to your PATH if not already:"
        echo ""
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
    fi

    echo ""
    info "Quick start:"
    echo "  spore install ./claw.spore    # Install a .spore package"
    echo "  spore start claw              # Start the service"
    echo "  spore status                  # Check status"
    echo ""
}

# Main
main() {
    echo ""
    echo "  ╔═══════════════════════════════════════╗"
    echo "  ║   Spore — StarClaw Deployment System  ║"
    echo "  ║   Ultra-lightweight. Any device.       ║"
    echo "  ╚═══════════════════════════════════════╝"
    echo ""

    detect_platform
    detect_install_dir
    download
    download_hatchery
    setup_home
    verify
}

main

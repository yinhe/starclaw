#!/bin/bash
# StarClaw MCP Bridge - Auto Install Script (Linux / macOS)
# Usage: curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/deploy/setup-bridge.sh | bash

set -e

BRIDGE_PORT=9101
REPO="yinhe/starclaw"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="mcp-bridge"

echo "🌟 StarClaw MCP Bridge Installer"
echo "================================="

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux)  PLATFORM="linux" ;;
  darwin) PLATFORM="darwin" ;;
  *)      echo "❌ Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *)             echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET_NAME="mcp-bridge-${PLATFORM}-${GOARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"

echo "📦 OS: ${PLATFORM}, Arch: ${GOARCH}"
echo "📥 Downloading from: ${DOWNLOAD_URL}"

# Download
TMP_FILE=$(mktemp)
if command -v curl &>/dev/null; then
  curl -fsSL -o "$TMP_FILE" "$DOWNLOAD_URL"
elif command -v wget &>/dev/null; then
  wget -q -O "$TMP_FILE" "$DOWNLOAD_URL"
else
  echo "❌ curl or wget required"; exit 1
fi

# Install
echo "📂 Installing to ${INSTALL_DIR}/${BINARY_NAME}"
sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Verify
if ! "${INSTALL_DIR}/${BINARY_NAME}" -version 2>/dev/null; then
  echo "✅ Binary installed (version check not supported)"
fi

# Setup auto-start based on OS
if [ "$PLATFORM" = "linux" ] && command -v systemctl &>/dev/null; then
  echo "🔧 Setting up systemd service..."

  cat > /tmp/mcp-bridge.service <<EOF
[Unit]
Description=StarClaw MCP Bridge - Host Control Server
After=network.target
Documentation=https://github.com/${REPO}

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -port ${BRIDGE_PORT}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mcp-bridge

[Install]
WantedBy=multi-user.target
EOF

  sudo mv /tmp/mcp-bridge.service /etc/systemd/system/mcp-bridge.service
  sudo systemctl daemon-reload
  sudo systemctl enable mcp-bridge
  sudo systemctl restart mcp-bridge

  echo "✅ systemd service installed and started"
  echo "   Status: systemctl status mcp-bridge"

elif [ "$PLATFORM" = "darwin" ]; then
  echo "🔧 Setting up launchd service..."

  PLIST_PATH="$HOME/Library/LaunchAgents/com.starclaw.mcp-bridge.plist"
  cat > "$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.starclaw.mcp-bridge</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/${BINARY_NAME}</string>
    <string>-port</string>
    <string>${BRIDGE_PORT}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/mcp-bridge.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/mcp-bridge.err</string>
</dict>
</plist>
EOF

  launchctl unload "$PLIST_PATH" 2>/dev/null || true
  launchctl load "$PLIST_PATH"

  echo "✅ launchd service installed and started"
  echo "   Logs: /tmp/mcp-bridge.log"

else
  # Fallback: just start in background
  echo "🔧 Starting bridge in background..."
  nohup "${INSTALL_DIR}/${BINARY_NAME}" -port ${BRIDGE_PORT} > /tmp/mcp-bridge.log 2>&1 &
  echo "✅ Bridge started (PID: $!)"
fi

echo ""
echo "🎉 MCP Bridge is running on port ${BRIDGE_PORT}"
echo "   The Claw API will auto-detect and connect within 30 seconds."
echo "   To verify: curl http://localhost:${BRIDGE_PORT}/health"

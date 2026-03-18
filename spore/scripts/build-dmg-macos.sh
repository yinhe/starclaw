#!/bin/bash
# Build proper macOS DMGs using hdiutil (must run on macOS)
# Usage: ./build-dmg-macos.sh [version-tag] [releases-dir]
# Example: ./build-dmg-macos.sh v2026.0318.0738 /opt/spore/releases
set -e

VTAG="${1:?Usage: $0 <version-tag> [releases-dir]}"
RELEASES="${2:-/opt/spore/releases}"
TMPDIR="/tmp/spore-dmg-$$"
rm -rf "$TMPDIR"

if ! command -v hdiutil &>/dev/null; then
  echo "Error: hdiutil not found. This script must run on macOS."
  exit 1
fi

echo "=== Building macOS DMGs (hdiutil) ==="
echo "Version: $VTAG"
echo "Source:  $RELEASES"

for arch in arm64 amd64; do
  suffix="darwin-$arch"
  RAW="$RELEASES/StarClaw-Setup-${VTAG}-$suffix"

  if [ ! -f "$RAW" ]; then
    echo "  !! macOS $suffix binary not found: $RAW"
    continue
  fi

  echo "--- Building DMG for $suffix ---"

  # Create temp app directory
  APP_DIR="$TMPDIR/$suffix/StarClaw Installer"
  mkdir -p "$APP_DIR"

  # Copy binary
  cp "$RAW" "$APP_DIR/StarClaw-Setup"
  chmod +x "$APP_DIR/StarClaw-Setup"

  # Create install script
  cat > "$APP_DIR/Install StarClaw.command" << 'SCRIPT'
#!/bin/bash
cd "$(dirname "$0")"

# Remove macOS quarantine flags
xattr -cr . 2>/dev/null || true
xattr -d com.apple.quarantine ./StarClaw-Setup 2>/dev/null || true
chmod +x ./StarClaw-Setup 2>/dev/null || true

echo ""
echo "  ✨ StarClaw Installer"
echo ""

# Try to run the setup
./StarClaw-Setup "$@"
STATUS=$?

if [ $STATUS -ne 0 ]; then
  echo ""
  echo "  ❌ Setup exited with error code $STATUS"
  echo ""
  echo "  If macOS blocked the app, try:"
  echo "    1. Open System Settings → Privacy & Security"
  echo "    2. Scroll down and click 'Open Anyway' next to the StarClaw message"
  echo "    3. Or right-click the installer and choose 'Open'"
  echo ""
  echo "  Press Enter to close..."
  read -r
fi
SCRIPT
  chmod +x "$APP_DIR/Install StarClaw.command"

  # Create README
  cat > "$APP_DIR/README.txt" << README
StarClaw Setup — One-click AI Agent Platform Installer

How to install:
  1. Double-click "Install StarClaw.command"
  2. If macOS blocks it: right-click → Open
  3. Follow the browser-based setup wizard

Version: $VTAG
Architecture: $arch
Website: https://starclaw.me
README

  # Create DMG with hdiutil
  DMG_PATH="$RELEASES/StarClaw-Setup-${VTAG}-${suffix}.dmg"
  
  # First create a read-write DMG
  hdiutil create \
    -volname "StarClaw Installer" \
    -srcfolder "$APP_DIR" \
    -ov \
    -format UDZO \
    -imagekey zlib-level=9 \
    "$DMG_PATH"

  echo "  -> $(basename "$DMG_PATH") ($(du -h "$DMG_PATH" | cut -f1))"
done

# Cleanup
rm -rf "$TMPDIR"

echo ""
echo "=== DMG build complete ==="
ls -lh "$RELEASES"/StarClaw-Setup-${VTAG}-darwin-*.dmg 2>/dev/null

#!/bin/bash
# Package Spore releases into platform-specific formats (with version in filename)
# Usage: package-releases.sh [version-tag]
# Run on Nydus server after raw binaries are uploaded to /opt/spore/releases/
set -e

RELEASES="/opt/spore/releases"
VTAG="${1:-}"
TMPDIR="/tmp/spore-pkg"
rm -rf "$TMPDIR"

# Auto-detect version from uploaded files if not provided
if [ -z "$VTAG" ]; then
  VTAG=$(ls "$RELEASES"/StarClaw-Setup-v*.exe 2>/dev/null | head -1 | grep -oP 'v[\d.]+' || echo "")
fi
if [ -z "$VTAG" ]; then
  echo "Error: version tag not found. Usage: $0 v2026.0318.0738"
  exit 1
fi
echo "Version: $VTAG"

echo "=== Packaging Linux tar.gz ==="
LINUX_RAW="$RELEASES/StarClaw-Setup-${VTAG}-linux-amd64"
if [ ! -f "$LINUX_RAW" ]; then
  # Fallback: check for tar.gz already built by Windows script
  if [ -f "$RELEASES/StarClaw-Setup-${VTAG}-linux-amd64.tar.gz" ]; then
    echo "  -> Already packaged: StarClaw-Setup-${VTAG}-linux-amd64.tar.gz"
  else
    echo "  !! Linux raw binary not found: $LINUX_RAW"
  fi
else
  mkdir -p "$TMPDIR/starclaw-setup-linux"
  cp "$LINUX_RAW" "$TMPDIR/starclaw-setup-linux/StarClaw-Setup"
  chmod +x "$TMPDIR/starclaw-setup-linux/StarClaw-Setup"
  cat > "$TMPDIR/starclaw-setup-linux/install.sh" << 'SCRIPT'
#!/bin/bash
set -e
cd "$(dirname "$0")"
chmod +x ./StarClaw-Setup
echo "Starting StarClaw Setup..."
./StarClaw-Setup "$@"
SCRIPT
  chmod +x "$TMPDIR/starclaw-setup-linux/install.sh"
  tar czf "$RELEASES/StarClaw-Setup-${VTAG}-linux-amd64.tar.gz" -C "$TMPDIR" starclaw-setup-linux/
  echo "  -> StarClaw-Setup-${VTAG}-linux-amd64.tar.gz ($(du -h "$RELEASES/StarClaw-Setup-${VTAG}-linux-amd64.tar.gz" | cut -f1))"
fi

echo "=== Packaging macOS DMGs ==="
for arch in arm64 amd64; do
  suffix="darwin-$arch"
  RAW="$RELEASES/StarClaw-Setup-${VTAG}-$suffix"

  if [ ! -f "$RAW" ]; then
    echo "  !! macOS $suffix binary not found: $RAW"
    continue
  fi

  DMG_DIR="$TMPDIR/dmg-$suffix"
  mkdir -p "$DMG_DIR/StarClaw Setup"

  cp "$RAW" "$DMG_DIR/StarClaw Setup/StarClaw-Setup"
  chmod +x "$DMG_DIR/StarClaw Setup/StarClaw-Setup"

  cat > "$DMG_DIR/StarClaw Setup/Install StarClaw.command" << 'SCRIPT'
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
  chmod +x "$DMG_DIR/StarClaw Setup/Install StarClaw.command"

  DMG_OUT="$RELEASES/StarClaw-Setup-${VTAG}-$suffix.dmg"

  if command -v hdiutil &>/dev/null; then
    # macOS: create proper DMG with hdiutil
    hdiutil create -volname "StarClaw Installer" \
      -srcfolder "$DMG_DIR/StarClaw Setup/" \
      -ov -format UDZO -imagekey zlib-level=9 \
      "$DMG_OUT"
  else
    # Linux fallback: genisoimage (basic ISO, not true DMG)
    genisoimage -V "StarClaw" \
      -D -R -J -no-pad \
      -o "$DMG_OUT" \
      "$DMG_DIR/StarClaw Setup/"
  fi

  echo "  -> $(basename "$DMG_OUT") ($(du -h "$DMG_OUT" | cut -f1))"
done

echo "=== Done ==="
rm -rf "$TMPDIR"
echo ""
echo "Release artifacts:"
ls -lh "$RELEASES"/StarClaw-Setup-${VTAG}* 2>/dev/null

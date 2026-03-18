#!/bin/bash
# Package Spore releases into platform-specific formats
# Run on Nydus server after raw binaries are uploaded to /opt/spore/releases/
set -e

RELEASES="/opt/spore/releases"
TMPDIR="/tmp/spore-pkg"
rm -rf "$TMPDIR"

echo "=== Packaging Linux tar.gz ==="
mkdir -p "$TMPDIR/starclaw-setup-linux"
cp "$RELEASES/StarClaw-Setup-linux-amd64" "$TMPDIR/starclaw-setup-linux/StarClaw-Setup"
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
cat > "$TMPDIR/starclaw-setup-linux/README.txt" << 'README'
StarClaw Setup (Linux x86_64)

Quick start:
  chmod +x StarClaw-Setup
  ./StarClaw-Setup

The installer will open a browser-based setup wizard.
Use --cli flag for command-line mode: ./StarClaw-Setup --cli
README
cd "$TMPDIR"
tar czf "$RELEASES/StarClaw-Setup-linux-amd64.tar.gz" -C "$TMPDIR" starclaw-setup-linux/
echo "  -> StarClaw-Setup-linux-amd64.tar.gz ($(du -h "$RELEASES/StarClaw-Setup-linux-amd64.tar.gz" | cut -f1))"

echo "=== Packaging macOS DMGs ==="
for arch in arm64 amd64; do
  if [ "$arch" = "arm64" ]; then
    label="Apple Silicon"
    suffix="darwin-arm64"
  else
    label="Intel"
    suffix="darwin-amd64"
  fi

  DMG_DIR="$TMPDIR/dmg-$suffix"
  mkdir -p "$DMG_DIR/StarClaw Setup"

  cp "$RELEASES/StarClaw-Setup-$suffix" "$DMG_DIR/StarClaw Setup/StarClaw-Setup"
  chmod +x "$DMG_DIR/StarClaw Setup/StarClaw-Setup"

  # Create install.command (double-clickable on macOS)
  cat > "$DMG_DIR/StarClaw Setup/Install StarClaw.command" << SCRIPT
#!/bin/bash
set -e
cd "\$(dirname "\$0")"
# Remove macOS quarantine attribute
xattr -d com.apple.quarantine ./StarClaw-Setup 2>/dev/null || true
echo "Starting StarClaw Setup..."
./StarClaw-Setup
SCRIPT
  chmod +x "$DMG_DIR/StarClaw Setup/Install StarClaw.command"

  cat > "$DMG_DIR/StarClaw Setup/README.txt" << README
StarClaw Setup (macOS $label)

Option 1: Double-click "Install StarClaw.command"
Option 2: Open Terminal and run: ./StarClaw-Setup

The installer will open a browser-based setup wizard.
Use --cli flag for command-line mode.
README

  # Create DMG using genisoimage (ISO9660 + Rock Ridge + Joliet)
  genisoimage -V "StarClaw" \
    -D -R -J \
    -no-pad \
    -o "$RELEASES/StarClaw-Setup-$suffix.dmg" \
    "$DMG_DIR/StarClaw Setup/"

  echo "  -> StarClaw-Setup-$suffix.dmg ($(du -h "$RELEASES/StarClaw-Setup-$suffix.dmg" | cut -f1))"
done

echo "=== Done ==="
rm -rf "$TMPDIR"
ls -lh "$RELEASES"/*.tar.gz "$RELEASES"/*.dmg 2>/dev/null

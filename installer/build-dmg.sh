#!/bin/bash
# TailClip DMG Builder
# Author: Toluwalase Mebaanne
#
# Builds arm64 binaries and packages them into a DMG installer.
# Run from the TailClip project root directory.
#
# Usage: ./installer/build-dmg.sh

set -e

# --- Configuration ---
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER_DIR="$PROJECT_ROOT/installer"
DIST_DIR="$INSTALLER_DIR/dist"
STAGING_DIR="$DIST_DIR/staging"
DMG_NAME="TailClip-Installer"
VOLUME_NAME="TailClip Installer"

echo "=== TailClip DMG Builder ==="
echo ""
echo "Project root: $PROJECT_ROOT"

# --- Step 1: Clean previous builds ---
echo "[1/5] Cleaning previous builds..."
rm -rf "$DIST_DIR"
mkdir -p "$STAGING_DIR"
echo "  Clean"

# --- Step 2: Build arm64 binaries ---
echo "[2/5] Building arm64 binaries..."

cd "$PROJECT_ROOT"

GOOS=darwin GOARCH=arm64 go build -o "$STAGING_DIR/tailclip-hub" ./hub/
echo "  Built tailclip-hub (darwin/arm64)"

GOOS=darwin GOARCH=arm64 go build -o "$STAGING_DIR/tailclip-agent" ./agent/
echo "  Built tailclip-agent (darwin/arm64)"

# --- Step 3: Copy installer scripts ---
echo "[3/5] Copying installer scripts..."
cp "$INSTALLER_DIR/Install TailClip.command" "$STAGING_DIR/"
cp "$INSTALLER_DIR/Uninstall TailClip.command" "$STAGING_DIR/"
chmod +x "$STAGING_DIR/Install TailClip.command"
chmod +x "$STAGING_DIR/Uninstall TailClip.command"
echo "  Copied Install and Uninstall scripts"

# --- Step 4: Add README ---
echo "[4/5] Creating README..."
cat > "$STAGING_DIR/README.txt" << 'EOF'
TailClip — Clipboard Sync over Tailscale
=========================================

Getting Started:
1. Double-click "Install TailClip" to start the installer
2. Follow the on-screen prompts to configure your setup
3. That's it! TailClip will start automatically

To Uninstall:
  Double-click "Uninstall TailClip"

Requirements:
  - macOS (Apple Silicon)
  - Tailscale installed and running

Note: macOS may show a security warning since this app
isn't signed. Right-click → Open to bypass this.

Learn more: https://github.com/tolumebaanne/TailClip
EOF
echo "  Created README.txt"

# --- Step 5: Create DMG ---
echo "[5/5] Creating DMG..."

# Create DMG from staging directory
hdiutil create -volname "$VOLUME_NAME" \
    -srcfolder "$STAGING_DIR" \
    -ov \
    -format UDZO \
    "$DIST_DIR/$DMG_NAME.dmg"

echo ""
echo "=== Build Complete ==="
echo ""
echo "DMG created: $DIST_DIR/$DMG_NAME.dmg"
echo ""
echo "Contents:"
echo "  • tailclip-hub          (arm64 binary)"
echo "  • tailclip-agent        (arm64 binary)"
echo "  • Install TailClip      (interactive installer)"
echo "  • Uninstall TailClip    (uninstaller)"
echo "  • README.txt"

# Show file size
DMG_SIZE=$(du -h "$DIST_DIR/$DMG_NAME.dmg" | cut -f1)
echo ""
echo "DMG size: $DMG_SIZE"

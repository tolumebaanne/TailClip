#!/bin/bash
# TailClip DMG Builder
# Author: Toluwalase Mebaanne
#
# Builds arm64 binaries and packages them into a clean DMG installer.
# The DMG shows only the installer — binaries are hidden in .resources/.
#
# Usage: ./installer/build-dmg.sh

set -e

# --- Configuration ---
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER_DIR="$PROJECT_ROOT/installer"
DIST_DIR="$INSTALLER_DIR/dist"
STAGING_DIR="$DIST_DIR/staging"
RESOURCES_DIR="$STAGING_DIR/.resources"
DMG_NAME="TailClip-Installer"
VOLUME_NAME="TailClip"

echo "=== TailClip DMG Builder ==="
echo ""
echo "Project root: $PROJECT_ROOT"

# --- Step 1: Clean previous builds ---
echo "[1/5] Cleaning previous builds..."
rm -rf "$DIST_DIR"
mkdir -p "$RESOURCES_DIR"
echo "  Clean"

# --- Step 2: Build arm64 binaries ---
echo "[2/5] Building arm64 binaries..."

cd "$PROJECT_ROOT"

GOOS=darwin GOARCH=arm64 go build -o "$RESOURCES_DIR/tailclip-hub" ./hub/
echo "  Built tailclip-hub (darwin/arm64)"

GOOS=darwin GOARCH=arm64 go build -o "$RESOURCES_DIR/tailclip-agent" ./agent/
echo "  Built tailclip-agent (darwin/arm64)"

# --- Step 3: Copy installer and uninstaller into resources ---
echo "[3/5] Staging installer files..."
cp "$INSTALLER_DIR/Install TailClip.command" "$STAGING_DIR/"
cp "$INSTALLER_DIR/Uninstall TailClip.command" "$RESOURCES_DIR/"
chmod +x "$STAGING_DIR/Install TailClip.command"
chmod +x "$RESOURCES_DIR/Uninstall TailClip.command"
echo "  Staged Install TailClip.command (visible)"
echo "  Staged Uninstall TailClip.command (in .resources)"

# --- Step 4: Set Finder metadata ---
echo "[4/5] Configuring DMG appearance..."

# Create a background instructions file (hidden)
cat > "$RESOURCES_DIR/README.txt" << 'EOF'
TailClip — Clipboard Sync over Tailscale
=========================================

Double-click "Install TailClip" to start the interactive installer.

To uninstall later, run:
  ~/.config/tailclip/Uninstall TailClip.command

Or from Terminal:
  bash "$HOME/.config/tailclip/Uninstall TailClip.command"

Requirements:
  - macOS (Apple Silicon)
  - Tailscale installed and running

Learn more: https://github.com/tolumebaanne/TailClip
EOF
echo "  Created README"

# --- Step 5: Create DMG ---
echo "[5/5] Creating DMG..."

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
echo "User sees:  Install TailClip  (one file, double-click to start)"
echo "Hidden:     .resources/tailclip-hub, .resources/tailclip-agent"
echo "            .resources/Uninstall TailClip.command"

DMG_SIZE=$(du -h "$DIST_DIR/$DMG_NAME.dmg" | cut -f1)
echo ""
echo "DMG size: $DMG_SIZE"

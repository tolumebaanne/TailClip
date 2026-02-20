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

# --- Step 3: Create Installer and Uninstaller App Bundles ---
echo "[3/5] Creating App Bundles..."

create_app_bundle() {
    local script_path="$1"
    local dest_dir="$2"
    local app_name="$3"
    local bundle_id="com.tailclip.${app_name// /-}"

    local app_dir="$dest_dir/$app_name.app"
    mkdir -p "$app_dir/Contents/MacOS"

    # Copy script and make executable
    cp "$script_path" "$app_dir/Contents/MacOS/$app_name"
    chmod +x "$app_dir/Contents/MacOS/$app_name"

    # Create Info.plist for background execution
    cat > "$app_dir/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$app_name</string>
    <key>CFBundleIdentifier</key>
    <string>$bundle_id</string>
    <key>CFBundleName</key>
    <string>$app_name</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>LSUIElement</key>
    <true/>
</dict>
</plist>
EOF
}

create_app_bundle "$INSTALLER_DIR/Install TailClip.command" "$STAGING_DIR" "Install TailClip"
create_app_bundle "$INSTALLER_DIR/Uninstall TailClip.command" "$RESOURCES_DIR" "Uninstall TailClip"

echo "  Created Install TailClip.app (visible)"
echo "  Created Uninstall TailClip.app (in .resources)"

# --- Step 4: Set Finder metadata ---
echo "[4/5] Configuring DMG appearance..."

# Create a background instructions file (hidden)
cat > "$RESOURCES_DIR/README.txt" << 'EOF'
TailClip — Clipboard Sync over Tailscale
=========================================

Double-click "Install TailClip" to start the interactive installer.

To uninstall later:
  Open ~/.config/tailclip/Uninstall TailClip.app

Or from Terminal:
  open "$HOME/.config/tailclip/Uninstall TailClip.app"

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
echo "User sees:  Install TailClip.app  (one file, double-click to start)"
echo "Hidden:     .resources/tailclip-hub, .resources/tailclip-agent"
echo "            .resources/Uninstall TailClip.app"

DMG_SIZE=$(du -h "$DIST_DIR/$DMG_NAME.dmg" | cut -f1)
echo ""
echo "DMG size: $DMG_SIZE"

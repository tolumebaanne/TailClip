#!/bin/bash
# TailClip macOS Uninstaller
# Author: Toluwalase Mebaanne
#
# Removes TailClip components installed by the TailClip installer.
# Double-click this file to start uninstallation.

set -e

# --- Constants ----------------------------------------------------------------
CONFIG_DIR="$HOME/.config/tailclip"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
BIN_DIR="/usr/local/bin"
HUB_BINARY="tailclip-hub"
AGENT_BINARY="tailclip-agent"
HUB_PLIST="com.tailclip.hub"
AGENT_PLIST="com.tailclip.agent"

# --- Helper Functions ---------------------------------------------------------

dialog_yesno() {
    osascript -e "display dialog \"$1\" with title \"TailClip Uninstaller\" buttons {\"Cancel\", \"$2\"} default button \"$2\" with icon caution" 2>/dev/null
    return $?
}

success() {
    osascript -e "display dialog \"$1\" with title \"TailClip Uninstaller\" buttons {\"Done\"} default button \"Done\" with icon note" 2>/dev/null
}

# --- Confirm ------------------------------------------------------------------

echo "=== TailClip macOS Uninstaller ==="
echo ""

dialog_yesno "This will uninstall TailClip from your Mac.\n\nThe following will be removed:\n• Binaries from /usr/local/bin\n• LaunchAgents (auto-start)\n\nYou will be asked about config files separately." "Uninstall" || exit 0

# --- Step 1: Stop and Remove LaunchAgents ------------------------------------

echo "[1/4] Stopping services..."

if [ -f "$LAUNCH_AGENTS_DIR/${HUB_PLIST}.plist" ]; then
    launchctl unload "$LAUNCH_AGENTS_DIR/${HUB_PLIST}.plist" 2>/dev/null || true
    rm -f "$LAUNCH_AGENTS_DIR/${HUB_PLIST}.plist"
    echo "  Removed hub LaunchAgent"
fi

if [ -f "$LAUNCH_AGENTS_DIR/${AGENT_PLIST}.plist" ]; then
    launchctl unload "$LAUNCH_AGENTS_DIR/${AGENT_PLIST}.plist" 2>/dev/null || true
    rm -f "$LAUNCH_AGENTS_DIR/${AGENT_PLIST}.plist"
    echo "  Removed agent LaunchAgent"
fi

# Also remove legacy plist if it exists from manual setup
if [ -f "$LAUNCH_AGENTS_DIR/com.tailclip.agent.plist" ]; then
    launchctl unload "$LAUNCH_AGENTS_DIR/com.tailclip.agent.plist" 2>/dev/null || true
    rm -f "$LAUNCH_AGENTS_DIR/com.tailclip.agent.plist"
    echo "  Removed legacy agent LaunchAgent"
fi

# --- Step 2: Remove Binaries ------------------------------------------------

echo "[2/4] Removing binaries..."

REMOVE_CMD=""
if [ -f "$BIN_DIR/$HUB_BINARY" ]; then
    REMOVE_CMD="rm -f '$BIN_DIR/$HUB_BINARY'"
    echo "  Removing $BIN_DIR/$HUB_BINARY"
fi
if [ -f "$BIN_DIR/$AGENT_BINARY" ]; then
    if [ -n "$REMOVE_CMD" ]; then
        REMOVE_CMD="$REMOVE_CMD && rm -f '$BIN_DIR/$AGENT_BINARY'"
    else
        REMOVE_CMD="rm -f '$BIN_DIR/$AGENT_BINARY'"
    fi
    echo "  Removing $BIN_DIR/$AGENT_BINARY"
fi

if [ -n "$REMOVE_CMD" ]; then
    osascript -e "do shell script \"$REMOVE_CMD\" with administrator privileges" 2>/dev/null
fi

# --- Step 3: Config Files (Optional) -----------------------------------------

echo "[3/4] Config files..."

if [ -d "$CONFIG_DIR" ]; then
    if dialog_yesno "Do you also want to remove your TailClip config files and database?\n\nLocation: $CONFIG_DIR\n\nChoose Cancel to keep them (useful if reinstalling)." "Remove Configs" 2>/dev/null; then
        rm -rf "$CONFIG_DIR"
        echo "  Removed $CONFIG_DIR"
    else
        echo "  Config files preserved at $CONFIG_DIR"
    fi
else
    echo "  No config directory found"
fi

# --- Step 4: Done -------------------------------------------------------------

echo "[4/4] Uninstallation complete!"
echo ""

success "TailClip has been uninstalled.\n\nAll services have been stopped and binaries removed."
echo "=== Uninstallation Complete ==="

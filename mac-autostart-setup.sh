#!/bin/bash
# Mac LaunchAgent Setup for TailClip Agent
# Author: Toluwalase Mebaanne
#
# Installs a LaunchAgent so the TailClip agent starts automatically
# when the user logs in and restarts if it crashes.
#
# Usage: ./mac-autostart-setup.sh
# Run from the TailClip project root directory.

set -e

# --- Configuration ---
INSTALL_DIR="$(cd "$(dirname "$0")" && pwd)"
PLIST_NAME="com.tailclip.agent"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"

echo "=== TailClip Mac Agent Autostart Setup ==="
echo ""
echo "Install dir: $INSTALL_DIR"

# Step 1: Build agent binary
echo "[1/4] Building agent binary..."
if [ ! -d "bin" ]; then
    mkdir -p bin
fi
go build -o bin/agent ./agent/
echo "  Agent binary built: bin/agent"

# Step 2: Verify config exists
echo "[2/4] Checking agent-config.json..."
if [ ! -f "$INSTALL_DIR/agent-config.json" ]; then
    echo "  ERROR: agent-config.json not found in $INSTALL_DIR"
    echo "  Copy agent.config.example.json to agent-config.json and edit it first."
    exit 1
fi
echo "  Config found: agent-config.json"

# Step 3: Create LaunchAgent plist
echo "[3/4] Creating LaunchAgent plist..."

# Unload existing service if present
if launchctl list "$PLIST_NAME" &>/dev/null; then
    echo "  Unloading existing service..."
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
fi

mkdir -p "$HOME/Library/LaunchAgents"

cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_NAME}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/bin/agent</string>
        <string>${INSTALL_DIR}/agent-config.json</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${INSTALL_DIR}</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>ThrottleInterval</key>
    <integer>5</integer>

    <key>StandardOutPath</key>
    <string>${INSTALL_DIR}/logs/agent-stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${INSTALL_DIR}/logs/agent-stderr.log</string>
</dict>
</plist>
EOF
echo "  Created $PLIST_PATH"

# Step 4: Load and start the service
echo "[4/4] Loading and starting service..."
mkdir -p "$INSTALL_DIR/logs"
launchctl load "$PLIST_PATH"
sleep 2

# Verify it's running
if launchctl list "$PLIST_NAME" &>/dev/null; then
    echo "  Service is running!"
else
    echo "  WARNING: Service may not have started. Check logs."
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "The agent will now start automatically on login."
echo ""
echo "Useful commands:"
echo "  launchctl list $PLIST_NAME                    # Check status"
echo "  launchctl stop $PLIST_NAME                    # Stop agent"
echo "  launchctl start $PLIST_NAME                   # Start agent"
echo "  launchctl unload $PLIST_PATH                  # Disable autostart"
echo "  tail -f $INSTALL_DIR/logs/agent-stdout.log    # Follow logs"

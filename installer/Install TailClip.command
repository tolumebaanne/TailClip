#!/bin/bash
# TailClip macOS Installer
# Author: Toluwalase Mebaanne
#
# Interactive installer for TailClip clipboard sync.
# Uses native macOS dialogs (osascript) to collect configuration.
# Double-click this file to start installation.

set -e

# Redirect all output to log file to avoid terminal clutter
exec > /tmp/tailclip_install.log 2>&1

# --- Constants ----------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_DIR="$HOME/.config/tailclip"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
BIN_DIR="/usr/local/bin"
HUB_BINARY="tailclip-hub"
AGENT_BINARY="tailclip-agent"
HUB_PLIST="com.tailclip.hub"
AGENT_PLIST="com.tailclip.agent"

# --- Helper Functions ---------------------------------------------------------

dialog() {
    osascript -e "display dialog \"$1\" with title \"TailClip Installer\" buttons {\"$2\"} default button \"$2\" with icon note" 2>/dev/null
}

dialog_yesno() {
    osascript -e "display dialog \"$1\" with title \"TailClip Installer\" buttons {\"Cancel\", \"$2\"} default button \"$2\" with icon note" 2>/dev/null
    return $?
}

input_dialog() {
    local prompt="$1"
    local default="$2"
    osascript -e "set result to display dialog \"$prompt\" with title \"TailClip Installer\" default answer \"$default\" with icon note" \
              -e "return text returned of result" 2>/dev/null
}

choose_dialog() {
    local prompt="$1"
    shift
    local items=""
    for item in "$@"; do
        items="$items\"$item\", "
    done
    items="${items%, }"
    osascript -e "choose from list {$items} with title \"TailClip Installer\" with prompt \"$prompt\" without multiple selections allowed" 2>/dev/null
}

fail() {
    osascript -e "display dialog \"$1\" with title \"TailClip Installer\" buttons {\"OK\"} default button \"OK\" with icon stop" 2>/dev/null
    exit 1
}

success() {
    osascript -e "display dialog \"$1\" with title \"TailClip Installer\" buttons {\"Done\"} default button \"Done\" with icon note" 2>/dev/null
}

warn_dialog() {
    osascript -e "display dialog \"$1\" with title \"TailClip Installer\" buttons {\"OK\"} default button \"OK\" with icon caution" 2>/dev/null
}

notify() {
    osascript -e "display notification \"$1\" with title \"TailClip Installer\"" 2>/dev/null
}

# Validates an IP address: must be 4 octets of digits (0-255), no placeholders.
is_valid_ip() {
    local ip="$1"
    # Reject empty
    if [ -z "$ip" ]; then return 1; fi
    # Reject placeholders containing x
    if echo "$ip" | grep -qi 'x'; then return 1; fi
    # Reject anything that isn't digits and dots
    if ! echo "$ip" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then return 1; fi
    return 0
}

# --- Preflight ----------------------------------------------------------------

echo "=== TailClip macOS Installer ==="
echo ""

# Check binaries exist in the .resources directory
if [[ "$SCRIPT_DIR" == *"/Contents/MacOS" ]]; then
    # Running inside .app bundle from DMG
    RESOURCES_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)/.resources"
else
    # Direct execution
    RESOURCES_DIR="$SCRIPT_DIR/.resources"
fi

if [ ! -d "$RESOURCES_DIR" ]; then
    fail "Error: Resources not found.\n\nPlease run the installer from the mounted TailClip DMG."
fi
if [ ! -f "$RESOURCES_DIR/$HUB_BINARY" ] && [ ! -f "$RESOURCES_DIR/$AGENT_BINARY" ]; then
    fail "Error: No TailClip binaries found.\n\nPlease run the installer from the mounted TailClip DMG."
fi

# --- Step 1: Welcome ---------------------------------------------------------

dialog_yesno "Welcome to TailClip!\n\nTailClip syncs your clipboard across devices over Tailscale VPN.\n\nThis installer will:\n• Install selected components to /usr/local/bin\n• Create config files in ~/.config/tailclip\n• Set up auto-start via LaunchAgents\n\nRequires: Tailscale installed and running." "Continue" || exit 0

# --- Step 2: Component Selection ----------------------------------------------

COMPONENT=$(choose_dialog "What would you like to install?" "Agent Only (recommended for most devices)" "Hub Only (central server)" "Both Hub and Agent")

if [ "$COMPONENT" = "false" ] || [ -z "$COMPONENT" ]; then
    echo "Installation cancelled."
    exit 0
fi

INSTALL_HUB=false
INSTALL_AGENT=false

case "$COMPONENT" in
    "Agent Only"*) INSTALL_AGENT=true ;;
    "Hub Only"*)   INSTALL_HUB=true ;;
    "Both Hub"*)   INSTALL_HUB=true; INSTALL_AGENT=true ;;
esac

echo "Selected: $COMPONENT"
echo "  Hub: $INSTALL_HUB"
echo "  Agent: $INSTALL_AGENT"

# --- Step 3: Collect Configuration --------------------------------------------

# Auth token (always needed)
AUTH_TOKEN=$(input_dialog "Enter your shared auth token.\n\nThis must match across hub and all agents.\n\nGenerate one with: openssl rand -hex 32" "")
if [ -z "$AUTH_TOKEN" ]; then
    fail "Auth token is required. Installation cancelled."
fi

# Hub configuration
if [ "$INSTALL_HUB" = true ]; then
    while true; do
        HUB_LISTEN_IP=$(input_dialog "Hub: Enter the IP address to listen on.\n\n• 0.0.0.0 = all interfaces\n• 127.0.0.1 = localhost only\n• Your Tailscale IP = Tailnet only\n\n(Find your Tailscale IP with: tailscale ip -4)" "0.0.0.0")
        if [ -z "$HUB_LISTEN_IP" ]; then HUB_LISTEN_IP="0.0.0.0"; break; fi
        if is_valid_ip "$HUB_LISTEN_IP"; then break; fi
        warn_dialog "Invalid IP address: $HUB_LISTEN_IP\n\nPlease enter a valid IP like 0.0.0.0, 127.0.0.1, or your Tailscale IP (e.g., 100.64.0.1).\n\nDo not use placeholders like 100.x.x.x"
    done

    HUB_LISTEN_PORT=$(input_dialog "Hub: Enter the port number to listen on." "8080")
    if [ -z "$HUB_LISTEN_PORT" ]; then HUB_LISTEN_PORT="8080"; fi
fi

# Agent configuration
if [ "$INSTALL_AGENT" = true ]; then
    DEVICE_NAME=$(input_dialog "Agent: Enter a name for this device.\n\n(e.g., MacBook Air, Work iMac)" "$(scutil --get ComputerName 2>/dev/null || echo 'My Mac')")
    if [ -z "$DEVICE_NAME" ]; then DEVICE_NAME="My Mac"; fi

    # Generate device ID from name: lowercase, replace spaces with hyphens
    DEVICE_ID=$(echo "$DEVICE_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd 'a-z0-9-')

    if [ "$INSTALL_HUB" = true ]; then
        # If hub is also being installed, agent connects to localhost
        HUB_URL="http://127.0.0.1:${HUB_LISTEN_PORT}"
        dialog "Agent will connect to the local hub at:\n$HUB_URL" "OK"
    else
        while true; do
            HUB_URL=$(input_dialog "Agent: Enter the hub URL.\n\nFormat: http://<tailscale-ip>:<port>\n\nExample: http://100.68.33.103:8080\n\n(Find the hub's Tailscale IP with: tailscale ip -4 on the hub machine)" "")
            if [ -z "$HUB_URL" ]; then
                fail "Hub URL is required. Installation cancelled."
            fi
            # Extract IP from URL and validate
            URL_IP=$(echo "$HUB_URL" | sed -E 's|https?://||' | sed 's|:[0-9]*$||')
            if is_valid_ip "$URL_IP"; then break; fi
            warn_dialog "Invalid hub URL: $HUB_URL\n\nThe IP address '$URL_IP' is not valid.\n\nPlease enter a real IP address like:\nhttp://100.68.33.103:8080\n\nDo not use placeholders like 100.x.x.x"
        done
    fi
fi

# --- Step 4: Confirm Installation --------------------------------------------

SUMMARY="Ready to install TailClip:\n\n"
if [ "$INSTALL_HUB" = true ]; then
    SUMMARY="${SUMMARY}Hub:\n  • Binary: $BIN_DIR/$HUB_BINARY\n  • Listen: $HUB_LISTEN_IP:$HUB_LISTEN_PORT\n\n"
fi
if [ "$INSTALL_AGENT" = true ]; then
    SUMMARY="${SUMMARY}Agent:\n  • Binary: $BIN_DIR/$AGENT_BINARY\n  • Device: $DEVICE_NAME ($DEVICE_ID)\n  • Hub: $HUB_URL\n\n"
fi
SUMMARY="${SUMMARY}Config: $CONFIG_DIR/\nAutostart: LaunchAgents"

dialog_yesno "$SUMMARY" "Install" || exit 0

# --- Step 5: Install Binaries ------------------------------------------------

echo ""
notify "[1/4] Installing binaries..."
echo "[1/4] Installing binaries..."

# Request sudo once
osascript -e 'do shell script "mkdir -p /usr/local/bin" with administrator privileges' 2>/dev/null

if [ "$INSTALL_HUB" = true ]; then
    osascript -e "do shell script \"cp '$RESOURCES_DIR/$HUB_BINARY' '$BIN_DIR/$HUB_BINARY' && chmod +x '$BIN_DIR/$HUB_BINARY'\" with administrator privileges" 2>/dev/null
    echo "  Installed $BIN_DIR/$HUB_BINARY"
fi

if [ "$INSTALL_AGENT" = true ]; then
    osascript -e "do shell script \"cp '$RESOURCES_DIR/$AGENT_BINARY' '$BIN_DIR/$AGENT_BINARY' && chmod +x '$BIN_DIR/$AGENT_BINARY'\" with administrator privileges" 2>/dev/null
    echo "  Installed $BIN_DIR/$AGENT_BINARY"
fi

# --- Step 6: Generate Config Files -------------------------------------------

notify "[2/4] Creating config files..."
echo "[2/4] Creating config files..."
mkdir -p "$CONFIG_DIR"

if [ "$INSTALL_HUB" = true ]; then
    cat > "$CONFIG_DIR/hub-config.json" << HUBEOF
{
    "listen_ip": "$HUB_LISTEN_IP",
    "listen_port": $HUB_LISTEN_PORT,
    "auth_token": "$AUTH_TOKEN",
    "sqlite_path": "$CONFIG_DIR/tailclip.db",
    "history_limit": 1000,
    "retention_days": 30
}
HUBEOF
    echo "  Created $CONFIG_DIR/hub-config.json"
fi

if [ "$INSTALL_AGENT" = true ]; then
    cat > "$CONFIG_DIR/agent-config.json" << AGENTEOF
{
    "device_id": "$DEVICE_ID",
    "device_name": "$DEVICE_NAME",
    "hub_url": "$HUB_URL",
    "auth_token": "$AUTH_TOKEN",
    "enabled": true,
    "poll_interval_ms": 1000,
    "notify_enabled": true
}
AGENTEOF
    echo "  Created $CONFIG_DIR/agent-config.json"
fi

# --- Step 7: Create LaunchAgents ---------------------------------------------

notify "[3/4] Setting up auto-start..."
echo "[3/4] Setting up auto-start..."
mkdir -p "$LAUNCH_AGENTS_DIR"

if [ "$INSTALL_HUB" = true ]; then
    # Unload existing if present
    launchctl unload "$LAUNCH_AGENTS_DIR/${HUB_PLIST}.plist" 2>/dev/null || true

    cat > "$LAUNCH_AGENTS_DIR/${HUB_PLIST}.plist" << HUBPLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${HUB_PLIST}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${BIN_DIR}/${HUB_BINARY}</string>
        <string>${CONFIG_DIR}/hub-config.json</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${CONFIG_DIR}</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>ThrottleInterval</key>
    <integer>5</integer>

    <key>StandardOutPath</key>
    <string>${CONFIG_DIR}/hub-stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${CONFIG_DIR}/hub-stderr.log</string>
</dict>
</plist>
HUBPLISTEOF

    launchctl load "$LAUNCH_AGENTS_DIR/${HUB_PLIST}.plist"
    echo "  Hub LaunchAgent installed and started"
fi

if [ "$INSTALL_AGENT" = true ]; then
    # Unload existing if present
    launchctl unload "$LAUNCH_AGENTS_DIR/${AGENT_PLIST}.plist" 2>/dev/null || true

    # If hub is also installed, give it a moment to start
    if [ "$INSTALL_HUB" = true ]; then
        sleep 2
    fi

    cat > "$LAUNCH_AGENTS_DIR/${AGENT_PLIST}.plist" << AGENTPLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${AGENT_PLIST}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${BIN_DIR}/${AGENT_BINARY}</string>
        <string>${CONFIG_DIR}/agent-config.json</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${CONFIG_DIR}</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>ThrottleInterval</key>
    <integer>5</integer>

    <key>StandardOutPath</key>
    <string>${CONFIG_DIR}/agent-stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${CONFIG_DIR}/agent-stderr.log</string>
</dict>
</plist>
AGENTPLISTEOF

    launchctl load "$LAUNCH_AGENTS_DIR/${AGENT_PLIST}.plist"
    echo "  Agent LaunchAgent installed and started"
fi

# --- Step 8: Done! ------------------------------------------------------------

notify "[4/4] Installing uninstaller..."
echo "[4/4] Installing uninstaller..."
if [ -d "$RESOURCES_DIR/Uninstall TailClip.app" ]; then
    cp -R "$RESOURCES_DIR/Uninstall TailClip.app" "$CONFIG_DIR/"
    echo "  Installed to $CONFIG_DIR/Uninstall TailClip.app"
elif [ -f "$RESOURCES_DIR/Uninstall TailClip.command" ]; then
    cp "$RESOURCES_DIR/Uninstall TailClip.command" "$CONFIG_DIR/"
    chmod +x "$CONFIG_DIR/Uninstall TailClip.command"
    echo "  Installed to $CONFIG_DIR/Uninstall TailClip.command"
fi

echo ""
echo "Installation complete!"
echo ""

DONE_MSG="TailClip has been installed successfully!\n\n"
if [ "$INSTALL_HUB" = true ]; then
    DONE_MSG="${DONE_MSG}Hub is running on $HUB_LISTEN_IP:$HUB_LISTEN_PORT\n"
fi
if [ "$INSTALL_AGENT" = true ]; then
    DONE_MSG="${DONE_MSG}Agent ($DEVICE_NAME) is syncing to $HUB_URL\n"
fi
DONE_MSG="${DONE_MSG}\nConfig: $CONFIG_DIR/\nLogs: $CONFIG_DIR/*.log\n\nTo uninstall later:\n  Open $CONFIG_DIR/Uninstall TailClip.app"

success "$DONE_MSG"
echo "=== Installation Complete ==="

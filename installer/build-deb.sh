#!/bin/bash
# TailClip Ubuntu DEB Builder
#
# Builds amd64 binaries and packages them into a clean DEB installer.
#
# Usage: ./installer/build-deb.sh

set -e

# --- Configuration ---
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER_DIR="$PROJECT_ROOT/installer"
BUILD_DIR="$INSTALLER_DIR/deb-build"
DEBIAN_DIR="$BUILD_DIR/DEBIAN"
BIN_DEST="$BUILD_DIR/usr/share/tailclip"
DIST_DIR="$PROJECT_ROOT/distribution"
DEB_NAME="TailClip-Ubuntu-amd64"
VERSION="1.0.0"

echo "=== TailClip DEB Builder ==="
echo ""

# --- Step 1: Clean previous builds ---
echo "[1/4] Cleaning previous builds..."
rm -rf "$BUILD_DIR"
mkdir -p "$DEBIAN_DIR"
mkdir -p "$BIN_DEST"
mkdir -p "$DIST_DIR"
echo "  Clean"

# --- Step 2: Build amd64 binaries ---
echo "[2/4] Building amd64 binaries..."
cd "$PROJECT_ROOT"

GOOS=linux GOARCH=amd64 go build -o "$BIN_DEST/tailclip-hub" ./hub/
echo "  Built tailclip-hub (linux/amd64)"

GOOS=linux GOARCH=amd64 go build -o "$BIN_DEST/tailclip-agent" ./agent/
echo "  Built tailclip-agent (linux/amd64)"

# --- Step 3: Create DEBIAN/control ---
echo "[3/4] Creating package metadata..."

cat > "$DEBIAN_DIR/control" << EOF
Package: tailclip
Version: $VERSION
Architecture: amd64
Maintainer: TailClip <support@tailclip.com>
Description: TailClip clipboard sync over Tailscale
 TailClip syncs your clipboard across devices over Tailscale VPN.
 Depends on tailscale.
EOF

# --- Step 4: Create DEBIAN/postinst ---
cat > "$DEBIAN_DIR/postinst" << 'EOF'
#!/bin/bash
set -e

# Reattach to terminal for interactive prompts
exec < /dev/tty > /dev/tty 2>&1 || true

# Helper for displaying errors
fail() {
    echo -e "\nError: $1\n"
    exit 1
}

# IP validation
is_valid_ip() {
    local ip="$1"
    if [ -z "$ip" ]; then return 1; fi
    if echo "$ip" | grep -qi 'x'; then return 1; fi
    if ! echo "$ip" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then return 1; fi
    return 0
}

echo ""
echo "======================================"
echo "    TailClip Ubuntu Installation      "
echo "======================================"
echo ""

# Determine the actual user (since postinst runs as root)
REAL_USER=${SUDO_USER:-root}
if [ "$REAL_USER" = "root" ]; then
    USER_HOME="/root"
else
    USER_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)
fi
CONFIG_DIR="$USER_HOME/.config/tailclip"
BIN_DIR="/usr/local/bin"

if [ ! -d "$USER_HOME" ]; then
    fail "Could not determine user home directory."
fi

# 1. Component Selection
while true; do
    echo "What would you like to install?"
    echo "  1) Agent Only (recommended for most devices)"
    echo "  2) Hub Only (central server)"
    echo "  3) Both Hub and Agent"
    read -p "Select an option [1/2/3]: " opt
    case "$opt" in
        1) INSTALL_AGENT=true; INSTALL_HUB=false; break ;;
        2) INSTALL_AGENT=false; INSTALL_HUB=true; break ;;
        3) INSTALL_AGENT=true; INSTALL_HUB=true; break ;;
        *) echo -e "\nInvalid option. Please enter 1, 2, or 3.\n" ;;
    esac
done

echo ""

# 2. Collect Configuration
while true; do
    read -p "Enter your shared auth token: " AUTH_TOKEN
    if [ -n "$AUTH_TOKEN" ]; then break; fi
    echo -e "Validation Error: Auth token cannot be empty.\n"
done

echo ""

if [ "$INSTALL_HUB" = true ]; then
    while true; do
        read -p "Hub Listen IP [0.0.0.0]: " HUB_LISTEN_IP
        HUB_LISTEN_IP=${HUB_LISTEN_IP:-0.0.0.0}
        if is_valid_ip "$HUB_LISTEN_IP"; then break; fi
        echo -e "Validation Error: Invalid IP address.\n"
    done
    
    read -p "Hub Listen Port [8080]: " HUB_LISTEN_PORT
    HUB_LISTEN_PORT=${HUB_LISTEN_PORT:-8080}
    echo ""
fi

if [ "$INSTALL_AGENT" = true ]; then
    DEFAULT_NAME=$(hostname)
    read -p "Agent Device Name [$DEFAULT_NAME]: " DEVICE_NAME
    DEVICE_NAME=${DEVICE_NAME:-$DEFAULT_NAME}
    
    DEVICE_ID=$(echo "$DEVICE_NAME" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9' | sed 's/ /-/g')
    
    if [ "$INSTALL_HUB" = true ]; then
        HUB_URL="http://127.0.0.1:${HUB_LISTEN_PORT}"
        echo "Agent will auto-connect to local hub at: $HUB_URL"
    else
        while true; do
            read -p "Hub URL (e.g., http://100.x.x.x:8080): " HUB_URL
            if [ -z "$HUB_URL" ]; then 
                echo -e "Validation Error: Hub URL cannot be empty.\n"
                continue
            fi
            URL_IP=$(echo "$HUB_URL" | sed -E 's|https?://||' | sed 's|:[0-9]*$||')
            if is_valid_ip "$URL_IP"; then break; fi
            echo -e "Validation Error: Invalid IP address in URL.\n"
        done
    fi
    echo ""
fi

echo "Installing TailClip..."

# 3. Copy Binaries
echo "  - Copying binaries to $BIN_DIR"
if [ "$INSTALL_HUB" = true ]; then
    cp /usr/share/tailclip/tailclip-hub "$BIN_DIR/"
    chmod +x "$BIN_DIR/tailclip-hub"
fi
if [ "$INSTALL_AGENT" = true ]; then
    cp /usr/share/tailclip/tailclip-agent "$BIN_DIR/"
    chmod +x "$BIN_DIR/tailclip-agent"
fi

# 4. Create configs
echo "  - Creating configuration files in $CONFIG_DIR"
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
fi

# Set ownership to real user
chown -R "$REAL_USER":"$REAL_USER" "$CONFIG_DIR"

# 5. Setup Systemd services
echo "  - Setting up systemd services"

if [ "$INSTALL_HUB" = true ]; then
    cat > /etc/systemd/system/tailclip-hub.service << SYSTEMDEOF
[Unit]
Description=TailClip Hub
After=network.target tailscaled.service

[Service]
ExecStart=$BIN_DIR/tailclip-hub "$CONFIG_DIR/hub-config.json"
WorkingDirectory=$CONFIG_DIR
User=$REAL_USER
Restart=always
RestartSec=5
StandardOutput=append:$CONFIG_DIR/hub-stdout.log
StandardError=append:$CONFIG_DIR/hub-stderr.log

[Install]
WantedBy=multi-user.target
SYSTEMDEOF
    
    systemctl enable tailclip-hub.service
    systemctl start tailclip-hub.service
fi

if [ "$INSTALL_AGENT" = true ]; then
    cat > /etc/systemd/system/tailclip-agent.service << SYSTEMDEOF
[Unit]
Description=TailClip Agent
After=network.target tailscaled.service

[Service]
ExecStart=$BIN_DIR/tailclip-agent "$CONFIG_DIR/agent-config.json"
WorkingDirectory=$CONFIG_DIR
User=$REAL_USER
Restart=always
RestartSec=5
StandardOutput=append:$CONFIG_DIR/agent-stdout.log
StandardError=append:$CONFIG_DIR/agent-stderr.log

[Install]
WantedBy=multi-user.target
SYSTEMDEOF
    
    systemctl enable tailclip-agent.service
    systemctl start tailclip-agent.service
fi

systemctl daemon-reload

echo ""
echo "======================================"
echo "    TailClip Installed Successfully   "
echo "======================================"
echo "Binaries: $BIN_DIR/tailclip-*"
echo "Configs:  $CONFIG_DIR/"
echo "Uninstaller built-in: sudo dpkg -r tailclip"
echo ""

exit 0
EOF
chmod 0755 "$DEBIAN_DIR/postinst"

# --- Step 5: Create DEBIAN/prerm ---
cat > "$DEBIAN_DIR/prerm" << 'EOF'
#!/bin/bash
set -e

echo ""
echo "======================================"
echo "    Uninstalling TailClip...          "
echo "======================================"

# 1. Stop and disable services
if systemctl is-active --quiet tailclip-hub.service 2>/dev/null; then
    echo "  - Stopping tailclip-hub service"
    systemctl stop tailclip-hub.service || true
fi
if systemctl is-enabled --quiet tailclip-hub.service 2>/dev/null; then
    systemctl disable tailclip-hub.service || true
fi

if systemctl is-active --quiet tailclip-agent.service 2>/dev/null; then
    echo "  - Stopping tailclip-agent service"
    systemctl stop tailclip-agent.service || true
fi
if systemctl is-enabled --quiet tailclip-agent.service 2>/dev/null; then
    systemctl disable tailclip-agent.service || true
fi

# 2. Remove systemd files
echo "  - Removing systemd unit files"
rm -f /etc/systemd/system/tailclip-*.service
systemctl daemon-reload

# 3. Remove binaries
echo "  - Removing binaries"
rm -f /usr/local/bin/tailclip-hub
rm -f /usr/local/bin/tailclip-agent

# 4. Remove all user configs and logs
echo "  - Scrubbing config files and logs"
for dir in /home/* /root; do
    if [ -d "$dir/.config/tailclip" ]; then
        rm -rf "$dir/.config/tailclip"
        echo "    Removed $dir/.config/tailclip"
    fi
done

echo ""
echo "TailClip has been completely removed."
echo ""
exit 0
EOF
chmod 0755 "$DEBIAN_DIR/prerm"

# --- Step 6: Build DEB ---
echo "[4/4] Creating DEB package..."
dpkg-deb --build "$BUILD_DIR" "$DIST_DIR/$DEB_NAME.deb"

echo ""
echo "=== Build Complete ==="
echo ""
echo "DEB created: $DIST_DIR/$DEB_NAME.deb"
echo ""


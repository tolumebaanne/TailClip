#!/bin/bash
# Ubuntu Autostart Setup for TailClip Hub & Agent
# Author: Toluwalase Mebaanne
#
# Installs systemd services so the hub and agent start on boot
# and restart automatically if they crash.
#
# Usage: sudo ./ubuntu-autostart-setup.sh
# Run from the TailClip project root directory.

set -e

# --- Configuration ---
INSTALL_DIR="/home/opti/TailClip"
USER="opti"
GROUP="opti"

echo "=== TailClip Systemd Setup ==="
echo ""

# Check for root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run with sudo."
    echo "Usage: sudo ./ubuntu-autostart-setup.sh"
    exit 1
fi

# Step 1: Build binaries
echo "[1/5] Building binaries..."
sudo -u "$USER" bash -c "cd $INSTALL_DIR && go build -o bin/hub ./hub/"
sudo -u "$USER" bash -c "cd $INSTALL_DIR && go build -o bin/agent ./agent/"
echo "  Built bin/hub and bin/agent"

# Step 2: Create hub systemd service
echo "[2/5] Creating tailclip-hub.service..."
cat > /etc/systemd/system/tailclip-hub.service << EOF
[Unit]
Description=TailClip Hub - Clipboard Sync Server
After=network.target tailscaled.service
Wants=tailscaled.service

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bin/hub $INSTALL_DIR/hub-ubuntu-config.json
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/home/opti

[Install]
WantedBy=multi-user.target
EOF
echo "  Created /etc/systemd/system/tailclip-hub.service"

# Step 3: Create agent systemd service
echo "[3/5] Creating tailclip-agent.service..."
cat > /etc/systemd/system/tailclip-agent.service << EOF
[Unit]
Description=TailClip Agent - Clipboard Sync Client
After=network.target tailscaled.service tailclip-hub.service
Wants=tailscaled.service

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bin/agent $INSTALL_DIR/agent-ubuntu-config.json
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
Environment=DISPLAY=:0
Environment=DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/home/opti

[Install]
WantedBy=multi-user.target
EOF
echo "  Created /etc/systemd/system/tailclip-agent.service"

# Step 4: Reload systemd and enable services
echo "[4/5] Enabling services..."
systemctl daemon-reload
systemctl enable tailclip-hub.service
systemctl enable tailclip-agent.service
echo "  Both services enabled (will start on boot)"

# Step 5: Start services
echo "[5/5] Starting services..."
systemctl start tailclip-hub.service
sleep 2
systemctl start tailclip-agent.service
echo "  Both services started"

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Useful commands:"
echo "  sudo systemctl status tailclip-hub     # Check hub status"
echo "  sudo systemctl status tailclip-agent   # Check agent status"
echo "  sudo journalctl -u tailclip-hub -f     # Follow hub logs"
echo "  sudo journalctl -u tailclip-agent -f   # Follow agent logs"
echo "  sudo systemctl restart tailclip-hub    # Restart hub"
echo "  sudo systemctl restart tailclip-agent  # Restart agent"

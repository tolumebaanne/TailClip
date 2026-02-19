#!/bin/bash
# Ubuntu Agent Setup Script for TailClip
# Author: Toluwalase Mebaanne
#
# Builds and starts the TailClip agent on Ubuntu.
# Run from the TailClip project root directory.

set -e

echo "=== TailClip Agent Setup for Ubuntu ==="
echo ""

# Step 1: Check/create bin directory
echo "[1/4] Checking bin/ directory..."
if [ ! -d "bin" ]; then
    mkdir -p bin
    echo "  Created bin/ directory"
else
    echo "  bin/ directory exists"
fi

# Step 2: Build the agent binary
echo "[2/4] Building agent binary..."
go build -o bin/agent ./agent/
echo "  Agent binary built: bin/agent"

# Step 3: Create agent config
echo "[3/4] Creating agent-ubuntu-config.json..."
cat > agent-ubuntu-config.json << 'EOF'
{
  "device_id": "ubuntu-desktop",
  "device_name": "Ubuntu Desktop",
  "hub_url": "http://100.68.33.103:8080",
  "auth_token": "test-token-12345",
  "enabled": true,
  "poll_interval_ms": 1000,
  "notify_enabled": true
}
EOF
echo "  Config created: agent-ubuntu-config.json"

# Step 4: Start the agent
echo "[4/4] Starting TailClip agent..."
echo ""
echo "=== Agent Starting ==="
exec ./bin/agent agent-ubuntu-config.json

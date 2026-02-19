#!/bin/bash
# Ubuntu Hub Setup Script for TailClip
# Author: Toluwalase Mebaanne
#
# Builds and starts the TailClip hub on Ubuntu.
# Run from the TailClip project root directory.

set -e

echo "=== TailClip Hub Setup for Ubuntu ==="
echo ""

# Step 1: Check/create bin directory
echo "[1/5] Checking bin/ directory..."
if [ ! -d "bin" ]; then
    mkdir -p bin
    echo "  Created bin/ directory"
else
    echo "  bin/ directory exists"
fi

# Step 2: Build the hub binary
echo "[2/5] Building hub binary..."
go build -o bin/hub ./hub/
echo "  Hub binary built: bin/hub"

# Step 3: Copy Ubuntu config template
echo "[3/5] Creating hub-config.json from template..."
cp hub-ubuntu-config.json hub-config.json
echo "  Copied hub-ubuntu-config.json -> hub-config.json"

# Step 4: Update config values
echo "[4/5] Configuring hub-config.json..."
sed -i 's/"listen_ip": ".*"/"listen_ip": "100.68.33.103"/' hub-config.json
sed -i 's/"auth_token": ".*"/"auth_token": "test-token-12345"/' hub-config.json
echo "  listen_ip: 100.68.33.103"
echo "  auth_token: set"

# Step 5: Start the hub
echo "[5/5] Starting TailClip hub..."
echo ""
echo "=== Hub Starting ==="
exec ./bin/hub hub-config.json

# TailClip Ubuntu Restore Instructions

This document provides instructions for Antigravity on how to completely reset and restore TailClip on an Ubuntu device.

## 1. Stop and Remove Existing Services

Stop the services, disable them, and remove the systemd unit files:

```bash
sudo systemctl stop tailclip-hub tailclip-agent || true
sudo systemctl disable tailclip-hub tailclip-agent || true
sudo rm -f /etc/systemd/system/tailclip-hub.service
sudo rm -f /etc/systemd/system/tailclip-agent.service
sudo systemctl daemon-reload
```

## 2. Remove Existing Binaries

Remove binaries from previous typical installation locations (including DEB install paths):

```bash
sudo rm -rf /usr/share/tailclip
sudo rm -f /usr/bin/tailclip-hub /usr/bin/tailclip-agent
sudo rm -f /usr/local/bin/tailclip-hub /usr/local/bin/tailclip-agent
rm -rf ~/TailClip/bin
```

## 3. Purge Configs, Logs, and Database

Clear out all configurations, database files, and system logs related to TailClip:

```bash
rm -rf ~/.config/tailclip
rm -rf ~/tailclip.log
rm -f ~/tailclip.db ~/tailclip.db-shm ~/tailclip.db-wal
sudo journalctl --vacuum-time=1s --unit=tailclip-hub.service
sudo journalctl --vacuum-time=1s --unit=tailclip-agent.service
```

## 4. Pull Latest Code

Navigate to the project directory and fetch the latest clean state:

```bash
cd ~/TailClip
git fetch --all
git reset --hard origin/main
git pull
```

## 5. Build Hub and Agent Binaries

Compile the Go binaries into the `bin/` directory:

```bash
cd ~/TailClip
mkdir -p bin
go build -o bin/tailclip-hub ./hub/cmd
go build -o bin/tailclip-agent ./agent/cmd
```

## 6. Restore Cleanly

Run the original Ubuntu setup script to safely reinstall the services using the newly built binaries and configuration:

```bash
cd ~/TailClip
chmod +x ubuntu-autostart-setup.sh
./ubuntu-autostart-setup.sh
```

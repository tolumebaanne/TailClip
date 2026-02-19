# TailClip

**Clipboard synchronization across devices over Tailscale VPN.**

*Author: Toluwalase Mebaanne*

---

## What Is TailClip?

TailClip automatically syncs your clipboard between all your devices — Mac, Linux, Windows — over your private [Tailscale](https://tailscale.com/) network. Copy on one device, paste on another. No cloud services, no third-party servers, no clipboard data leaving your Tailnet.

### Features

- **Automatic clipboard sync** — Copy text on any device, it appears on all others within ~1 second
- **Real-time push via WebSocket** — Near-instant delivery (no polling delay for incoming events)
- **Clipboard history** — Hub stores recent events in SQLite for catch-up after reconnection
- **Desktop notifications** — Optional alerts when clipboard content arrives from another device
- **Loop prevention** — Event caching prevents infinite sync cycles between devices
- **Secure by design** — Runs entirely within your Tailscale network with shared-secret auth
- **Cross-platform** — Agents run on macOS, Linux, and Windows

---

## Architecture

```
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│   Agent A    │◄──WS──► │     Hub      │◄──WS──► │   Agent B    │
│  (Mac)       │──HTTP──►│  (Ubuntu)    │◄──HTTP──│  (Windows)   │
│              │         │  SQLite DB   │         │              │
│ clipboard.go │         │  server.go   │         │ clipboard.go │
│ sync.go      │         │  storage.go  │         │ sync.go      │
│ notify.go    │         │  broadcast.go│         │ notify.go    │
└──────────────┘         └──────────────┘         └──────────────┘
```

**Hub** — Central server (runs on one machine in your Tailnet). Receives clipboard events via HTTP, stores them in SQLite, and broadcasts to all connected agents via WebSocket.

**Agent** — Runs on each device. Polls the local clipboard for changes, pushes new content to the hub, and receives updates from other devices over WebSocket.

---

## Requirements

- **Go 1.25+** (for building from source)
- **Tailscale** installed and running on all devices
- One machine designated as the hub (always-on, e.g., a home server or VPS)

---

## Project Structure

```
TailClip/
├── hub/                        # Hub server (central coordinator)
│   ├── main.go                 # Entry point, startup sequence
│   ├── server.go               # HTTP API handlers
│   ├── storage.go              # SQLite persistence layer
│   └── broadcast.go            # WebSocket broadcaster
├── agent/                      # Agent client (per-device)
│   ├── main.go                 # Entry point, polling loop
│   ├── clipboard.go            # Cross-platform clipboard I/O
│   ├── sync.go                 # Hub communication, loop prevention
│   └── notifications.go        # Desktop notifications
├── shared/                     # Shared libraries
│   ├── auth/token.go           # Authentication utilities
│   ├── config/config.go        # Configuration loading
│   ├── models/event.go         # Clipboard event model
│   ├── models/device.go        # Device registration model
│   └── handlers/               # Content-type handlers
├── hub.config.example.json     # Example hub configuration
├── agent.config.example.json   # Example agent configuration
└── README.md
```

---

## Build

```bash
# Clone the repository
git clone https://github.com/tmair/tailclip.git
cd tailclip

# Build the hub
go build -o bin/hub ./hub/

# Build the agent
go build -o bin/agent ./agent/
```

---

## Configuration

### Hub Configuration

```bash
cp hub.config.example.json hub-config.json
```

Edit `hub-config.json`:

```json
{
    "listen_ip": "0.0.0.0",
    "listen_port": 8080,
    "auth_token": "your-secret-token-here",
    "sqlite_path": "tailclip.db",
    "history_limit": 1000,
    "retention_days": 30
}
```

| Field | Description |
|-------|-------------|
| `listen_ip` | Bind address. Use `0.0.0.0` for all interfaces or your Tailscale IP for Tailnet-only |
| `listen_port` | TCP port (default: `8080`) |
| `auth_token` | **Required.** Shared secret — must match all agents. Generate with `openssl rand -hex 32` |
| `sqlite_path` | Database file location |
| `history_limit` | Max events to retain |
| `retention_days` | Days before old events are purged |

> **Tip:** You can also set the token via the `TAILCLIP_HUB_AUTH_TOKEN` environment variable to avoid storing secrets in the config file.

### Agent Configuration

```bash
cp agent.config.example.json agent-config.json
```

Edit `agent-config.json`:

```json
{
    "device_id": "macbook-air",
    "device_name": "MacBook Air",
    "hub_url": "http://100.64.0.1:8080",
    "auth_token": "your-secret-token-here",
    "enabled": true,
    "poll_interval_ms": 1000,
    "notify_enabled": true
}
```

| Field | Description |
|-------|-------------|
| `device_id` | Unique slug for this device (e.g., `macbook-air`, `work-desktop`) |
| `device_name` | Human-readable name shown in notifications and logs |
| `hub_url` | Hub URL using the hub machine's **Tailscale IP**. Find it with `tailscale ip -4` on the hub |
| `auth_token` | **Required.** Must match the hub's token |
| `enabled` | Set `false` to temporarily disable sync |
| `poll_interval_ms` | How often to check clipboard (ms). Lower = faster sync, more CPU. Default: `1000` |
| `notify_enabled` | Show desktop notifications on clipboard sync |

---

## Usage

### 1. Start the Hub

On your hub machine (e.g., Ubuntu server):

```bash
# Find its Tailscale IP (agents will connect to this)
tailscale ip -4

# Start the hub
./bin/hub hub-config.json
```

Expected output:
```
Hub config loaded from hub-config.json
Storage initialized at tailclip.db
Broadcaster initialized
Starting TailClip hub on 0.0.0.0:8080
Hub listening on 0.0.0.0:8080
```

### 2. Start Agents

On each device you want to sync:

```bash
# macOS / Linux
./bin/agent agent-config.json

# Windows (PowerShell)
.\bin\agent.exe agent-config.json
```

Expected output:
```
Agent config loaded: device=macbook-air (MacBook Air), hub=http://100.64.0.1:8080
Syncer initialized for hub http://100.64.0.1:8080
WebSocket receiver started
Clipboard polling started (interval: 1s)
```

### 3. Test It

1. Copy some text on Device A
2. Wait ~1 second
3. Paste on Device B — the text should be there!

---

## Environment Variables

| Variable | Overrides | Component |
|----------|-----------|-----------|
| `TAILCLIP_HUB_AUTH_TOKEN` | `auth_token` | Hub |
| `TAILCLIP_HUB_PORT` | `listen_port` | Hub |
| `TAILCLIP_AGENT_AUTH_TOKEN` | `auth_token` | Agent |
| `TAILCLIP_HUB_URL` | `hub_url` | Agent |
| `TAILCLIP_DEVICE_ID` | `device_id` | Agent |

---

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/clipboard/push` | Header | Push a clipboard event |
| `GET` | `/api/v1/history` | Header | Get recent clipboard events |
| `POST` | `/api/v1/device/register` | Header | Register/heartbeat a device |
| `GET` | `/api/v1/health` | None | Liveness check |

Authentication uses the `X-Auth-Token` header for HTTP endpoints and `?token=` query parameter for WebSocket connections.

---

## Roadmap

| Phase | Feature | Status |
|-------|---------|--------|
| **Phase 1** | Text clipboard sync | ✅ Complete |
| **Phase 2** | Image clipboard sync | 🔲 Planned |
| **Phase 3** | File/URI clipboard sync | 🔲 Planned |

---

## License

MIT License — see [LICENSE](LICENSE) for details.

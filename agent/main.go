// Author: Toluwalase Mebaanne
// Package main is the entry point for the TailClip agent.
//
// WHY a separate main.go:
// Keeps startup, shutdown, and the main event loop isolated from clipboard
// operations (clipboard.go), network sync (sync.go), and notifications
// (notifications.go). This separation means each concern can be tested and
// modified independently.

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tmair/tailclip/shared/config"
	"github.com/tmair/tailclip/shared/models"
)

// defaultConfigPath is the file path checked when no explicit path is given.
// WHY a constant: Single source of truth for the default config location.
const defaultConfigPath = "agent-config.json"

// pruneInterval controls how often the event cache is cleaned up.
// WHY 1 minute: Frequent enough to keep memory bounded, infrequent enough
// to avoid unnecessary lock contention on the cache mutex.
const pruneInterval = 1 * time.Minute

func main() {
	// --- Step 1: Load configuration -------------------------------------------
	// WHY load config first: The entire agent depends on knowing its device ID,
	// hub URL, auth token, and polling interval. If any required field is missing,
	// fail immediately with a clear message rather than panicking later.
	configPath := defaultConfigPath
	if len(os.Args) > 1 {
		// WHY allow CLI override: Useful for running multiple agent instances
		// during development or testing different configurations.
		configPath = os.Args[1]
	}

	cfg, err := config.LoadAgentConfig(configPath)
	if err != nil {
		log.Fatalf("FATAL: failed to load agent config from %s: %v", configPath, err)
	}
	log.Printf("Agent config loaded: device=%s (%s), hub=%s",
		cfg.DeviceID, cfg.DeviceName, cfg.HubURL)

	// --- Step 2: Check if agent is enabled ------------------------------------
	// WHY check early: If the user disabled the agent in config, exit cleanly
	// instead of starting goroutines and network connections for nothing.
	if !cfg.Enabled {
		log.Printf("Agent is disabled in config. Exiting.")
		return
	}

	// --- Step 3: Initialize syncer --------------------------------------------
	// WHY create syncer before starting loops: Both the polling loop and
	// WebSocket receiver need the syncer, so it must be ready first.
	syncer := NewSyncer(cfg.HubURL, cfg.AuthToken, cfg.DeviceID)
	log.Printf("Syncer initialized for hub %s", cfg.HubURL)

	// --- Step 4: Set up graceful shutdown -------------------------------------
	// WHY handle SIGINT and SIGTERM:
	// Without signal handling, Ctrl+C or a system kill would terminate the
	// process immediately, potentially mid-write to clipboard or mid-push
	// to the hub. Catching signals lets us log a clean shutdown message and
	// let deferred functions run.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// --- Step 5: Start WebSocket receiver in background ----------------------
	// WHY a separate goroutine: WebSocket reads block until a message arrives
	// or the connection breaks. Running it concurrently lets the clipboard
	// polling loop continue independently. The two paths are:
	//   - Local clipboard → hub (polling loop below)
	//   - Hub → local clipboard (WebSocket goroutine)
	wsDone := make(chan struct{})
	go func() {
		defer close(wsDone)
		connectAndReceive(syncer, cfg)
	}()
	log.Printf("WebSocket receiver started")

	// --- Step 6: Start clipboard polling loop ---------------------------------
	// WHY a ticker-based loop:
	// The clipboard has no cross-platform change notification API (see
	// clipboard.go header). Polling at a regular interval is the simplest
	// approach that works everywhere. The interval is configurable via
	// poll_interval_ms to let users balance latency vs. CPU usage.
	//
	// WHY poll_interval_ms matters:
	//   - Too low (e.g., 50ms): Unnecessary CPU usage, especially on laptops
	//     running on battery. The clipboard rarely changes more than a few
	//     times per minute in normal usage.
	//   - Too high (e.g., 10s): Unacceptable sync latency. Users expect
	//     clipboard content to appear on other devices within ~1 second.
	//   - Default 1000ms: Good balance for most users. Fast enough to feel
	//     "instant" (human reaction time is ~200ms), slow enough to be
	//     imperceptible on CPU monitors.
	pollInterval := cfg.GetPollInterval()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Track the last known clipboard hash to detect changes.
	// WHY hash comparison: Comparing hashes is cheaper than comparing full
	// clipboard text (which could be very large) and avoids storing the
	// entire previous clipboard content in memory.
	lastHash := GetClipboardHash()

	// Prune timer for event cache cleanup.
	pruneTicker := time.NewTicker(pruneInterval)
	defer pruneTicker.Stop()

	log.Printf("Clipboard polling started (interval: %s)", pollInterval)

	// --- Main event loop ------------------------------------------------------
	// WHY select over multiple channels:
	// Go's select statement lets us react to whichever event occurs first -
	// a clipboard poll tick, a cache prune tick, or a shutdown signal -
	// without busy-waiting or complex threading.
	for {
		select {
		case <-ticker.C:
			handleClipboardPoll(syncer, cfg, &lastHash)

		case <-pruneTicker.C:
			syncer.PruneCache()

		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			return

		case <-wsDone:
			// WHY restart on disconnect: WebSocket connections can drop due
			// to network changes, hub restarts, or Tailscale reconnections.
			// Rather than exiting, wait briefly and reconnect to maintain
			// real-time sync.
			log.Printf("WebSocket disconnected, reconnecting in 5s...")
			time.Sleep(5 * time.Second)
			wsDone = make(chan struct{})
			go func() {
				defer close(wsDone)
				connectAndReceive(syncer, cfg)
			}()
		}
	}
}

// handleClipboardPoll checks if the clipboard has changed and pushes to hub.
//
// WHY extract from the loop: Keeps the main select clean and makes the
// polling logic testable independently.
func handleClipboardPoll(syncer *Syncer, cfg *config.AgentConfig, lastHash *string) {
	currentHash := GetClipboardHash()

	// No change since last poll - nothing to do.
	if currentHash == "" || currentHash == *lastHash {
		return
	}

	// Update last known hash immediately.
	// WHY before pushing: If PushToHub is slow or fails, we don't want
	// the next poll to detect the same "change" again and retry immediately.
	*lastHash = currentHash

	// Check if this hash was recently synced FROM the hub.
	// WHY: When ReceiveFromHub writes to the clipboard, the next poll will
	// detect it as a "change". Without this check, we'd push it right back
	// to the hub, creating a loop.
	if syncer.IsEventCached(currentHash) {
		return
	}

	// Read the actual clipboard text for the event payload.
	text := ReadClipboard()
	if text == "" {
		return
	}

	event := &models.Event{
		EventID:        uuid.New().String(),
		SourceDeviceID: cfg.DeviceID,
		Timestamp:      time.Now().UTC(),
		ContentType:    "text",
		Text:           text,
	}
	event.SetTextHash()

	// Cache both the event ID and the text hash.
	// WHY cache text hash: When the hub broadcasts this event back and
	// ReceiveFromHub writes to clipboard, the poll loop will see a "new"
	// hash. Caching the hash lets us recognize it as our own content.
	syncer.CacheEvent(event.EventID)
	syncer.CacheEvent(event.TextHash)

	if err := syncer.PushToHub(event); err != nil {
		log.Printf("ERROR: failed to push to hub: %v", err)
	}
}

// connectAndReceive establishes a WebSocket connection and starts receiving.
//
// WHY a helper function: Encapsulates the connect-then-receive pattern so
// the reconnection logic in the main loop can call it cleanly.
func connectAndReceive(syncer *Syncer, cfg *config.AgentConfig) {
	conn, err := syncer.ConnectWebSocket()
	if err != nil {
		log.Printf("ERROR: WebSocket connection failed: %v", err)
		return
	}
	// Log connection details for debugging
	_ = fmt.Sprintf("Connected to %s", cfg.HubURL)
	syncer.ReceiveFromHub(conn, cfg.NotifyEnabled)
}

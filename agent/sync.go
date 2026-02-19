// Author: Toluwalase Mebaanne
// Package main provides hub communication for the TailClip agent.
//
// WHY separate sync logic from clipboard and main:
// Sync handles all network communication with the hub (pushing events,
// receiving WebSocket updates). Isolating it keeps clipboard I/O and
// startup/shutdown logic out of the network layer, making each file
// easier to test and reason about independently.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tmair/tailclip/shared/models"
)

// recentEventCache tracks recently seen event IDs to prevent sync loops.
//
// WHY an event cache is critical for loop prevention:
// Without it, this sequence creates an infinite loop:
//  1. Agent A copies text → pushes event X to hub
//  2. Hub broadcasts event X to Agent B via WebSocket
//  3. Agent B writes text to clipboard → detects "new" clipboard content
//  4. Agent B pushes event Y (same text) to hub
//  5. Hub broadcasts event Y back to Agent A → goto 1
//
// By caching recently seen event IDs, the agent can recognize events it has
// already processed (either because it created them or received them from the
// hub) and skip them, breaking the cycle.
//
// WHY a map with timestamps instead of a simple set:
// Events need to expire from the cache eventually, otherwise it grows without
// bound. Timestamps let us periodically prune old entries.
type recentEventCache struct {
	mu     sync.Mutex
	events map[string]time.Time // eventID → time first seen
	maxAge time.Duration        // how long to keep entries
}

// newRecentEventCache creates a cache that expires entries after maxAge.
// WHY configurable maxAge: Different deployment scenarios may need different
// retention. A typical value of 5 minutes is generous enough to handle
// retransmissions while keeping memory bounded.
func newRecentEventCache(maxAge time.Duration) *recentEventCache {
	return &recentEventCache{
		events: make(map[string]time.Time),
		maxAge: maxAge,
	}
}

// Add records an event ID as recently seen.
// WHY: Called both when we push an event (so we ignore our own broadcast)
// and when we receive one (so we don't re-push it).
func (c *recentEventCache) Add(eventID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events[eventID] = time.Now()
}

// Contains checks if an event ID has been seen recently.
// WHY: The sync loop calls this before writing to clipboard or pushing to hub
// to decide whether to skip the event.
func (c *recentEventCache) Contains(eventID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	seen, ok := c.events[eventID]
	if !ok {
		return false
	}

	// Expire stale entries on read - WHY: Lazy expiration avoids the need
	// for a background goroutine. Since Contains is called frequently from
	// the polling loop, stale entries get cleaned up naturally.
	if time.Since(seen) > c.maxAge {
		delete(c.events, eventID)
		return false
	}
	return true
}

// Prune removes all entries older than maxAge.
// WHY: Called periodically to prevent unbounded memory growth. Even with
// lazy expiration in Contains, entries for events we never check again
// would linger forever without explicit pruning.
func (c *recentEventCache) Prune() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, seen := range c.events {
		if now.Sub(seen) > c.maxAge {
			delete(c.events, id)
		}
	}
}

// Syncer handles all communication between the agent and the hub.
//
// WHY a struct instead of standalone functions:
// Groups the hub URL, auth token, device ID, and event cache together.
// This avoids passing 4+ parameters to every sync function and makes
// it easy to create test instances with mock configuration.
type Syncer struct {
	hubURL    string
	authToken string
	deviceID  string
	cache     *recentEventCache
	client    *http.Client
}

// NewSyncer creates a Syncer configured for the given hub.
//
// WHY set an HTTP client timeout:
// Without a timeout, a hung hub would block the agent's goroutine forever,
// preventing it from detecting new clipboard changes or recovering.
// 10 seconds is generous for a LAN/Tailnet round trip.
func NewSyncer(hubURL, authToken, deviceID string) *Syncer {
	return &Syncer{
		hubURL:    hubURL,
		authToken: authToken,
		deviceID:  deviceID,
		cache:     newRecentEventCache(5 * time.Minute),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PushToHub sends a clipboard event to the hub's push endpoint.
//
// WHY POST with JSON body:
// Matches the hub's handlePush endpoint contract. JSON is human-readable
// for debugging and supported natively by Go's standard library.
//
// WHY cache the event ID before pushing:
// When the hub broadcasts this event back over WebSocket, the agent needs
// to recognize it as its own and skip it. Adding to cache before the push
// (rather than after) prevents a race where the broadcast arrives before
// the cache is updated.
func (s *Syncer) PushToHub(event *models.Event) error {
	// Cache event ID BEFORE pushing to prevent sync loops.
	// WHY before: The hub may broadcast the event back faster than we
	// return from this function, especially on a fast LAN.
	s.cache.Add(event.EventID)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	pushURL := fmt.Sprintf("%s/api/v1/clipboard/push", s.hubURL)
	req, err := http.NewRequest(http.MethodPost, pushURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", s.authToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("push request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("hub returned status %d on push", resp.StatusCode)
	}

	log.Printf("Pushed event %s to hub", event.EventID)
	return nil
}

// ConnectWebSocket establishes a WebSocket connection to the hub for
// real-time event delivery.
//
// WHY WebSocket for receiving (instead of polling):
// Polling /api/v1/history at the configured interval would work but has
// two drawbacks:
//  1. Latency: up to one full poll interval before the agent sees new events.
//  2. Wasted requests: most polls return no new events, burning CPU and network.
//
// A persistent WebSocket connection lets the hub push events the instant they
// arrive, giving near-zero latency with zero wasted requests.
//
// WHY pass the token as a query parameter:
// The WebSocket handshake is a standard HTTP GET request, and many client
// libraries (including gorilla/websocket) don't support custom headers on the
// upgrade request reliably across all platforms. Using ?token=<value> is the
// widely accepted workaround for WebSocket authentication (see shared/auth/token.go).
func (s *Syncer) ConnectWebSocket() (*websocket.Conn, error) {
	// Build WebSocket URL by replacing http(s) with ws(s).
	// WHY: The gorilla/websocket dialer expects a ws:// or wss:// scheme.
	wsURL, err := url.Parse(s.hubURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hub URL: %w", err)
	}

	switch wsURL.Scheme {
	case "https":
		wsURL.Scheme = "wss"
	default:
		wsURL.Scheme = "ws"
	}
	wsURL.Path = "/api/v1/ws"
	wsURL.RawQuery = fmt.Sprintf("token=%s&device_id=%s",
		url.QueryEscape(s.authToken),
		url.QueryEscape(s.deviceID))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket dial failed: %w", err)
	}

	log.Printf("WebSocket connected to hub")
	return conn, nil
}

// ReceiveFromHub listens on a WebSocket connection and processes incoming
// clipboard events. It writes synced content to the local clipboard and
// optionally shows a desktop notification.
//
// WHY run in its own goroutine:
// WebSocket reads are blocking. Running ReceiveFromHub in a separate goroutine
// lets the main polling loop continue detecting local clipboard changes
// independently. The two paths (local→hub, hub→local) run concurrently.
//
// WHY the notifyEnabled parameter:
// Keeps notification policy at the caller level (main.go reads config).
// This function shouldn't import or depend on config directly - it just
// does what it's told.
func (s *Syncer) ReceiveFromHub(conn *websocket.Conn, notifyEnabled bool) {
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// WHY log and return: A read error means the connection is dead
			// (closed by hub, network failure, etc.). The main loop will
			// detect the goroutine exit and attempt to reconnect.
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var event models.Event
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("WARN: failed to unmarshal WebSocket event: %v", err)
			continue
		}

		log.Printf("WebSocket received event: id=%s source=%s", event.EventID, event.SourceDeviceID)

		// Skip events from ourselves - WHY: Even though the hub skips the
		// source device in Broadcast, belt-and-suspenders defense prevents
		// loops if the hub logic ever changes or has a bug.
		if event.SourceDeviceID == s.deviceID {
			log.Printf("Skipping own event %s", event.EventID)
			continue
		}

		// Skip events we've already processed - WHY: Prevents duplicate
		// clipboard writes if the same event arrives via both WebSocket
		// and a history poll.
		if s.cache.Contains(event.EventID) {
			continue
		}

		// Cache before writing to clipboard - WHY: The clipboard write
		// will trigger a change detection in the polling loop. If the
		// event is already cached, the poll loop will skip it instead
		// of pushing it back to the hub.
		s.cache.Add(event.EventID)

		if err := WriteClipboard(event.Text); err != nil {
			log.Printf("ERROR: failed to write synced clipboard: %v", err)
			continue
		}

		log.Printf("Synced clipboard from device %s (event %s)",
			event.SourceDeviceID, event.EventID)

		if notifyEnabled {
			// Truncate text preview for notification readability.
			preview := event.Text
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			ShowNotification(event.SourceDeviceID, preview)
		}
	}
}

// IsEventCached checks if an event ID has been recently seen.
// WHY: Exposed for use by the main polling loop to determine whether a
// clipboard change was caused by a sync (and should be skipped) or by
// the user (and should be pushed to the hub).
func (s *Syncer) IsEventCached(eventID string) bool {
	return s.cache.Contains(eventID)
}

// CacheEvent records an event ID as recently seen.
// WHY: Exposed so the polling loop can cache event IDs for clipboard
// content hashes to track what it has already pushed.
func (s *Syncer) CacheEvent(eventID string) {
	s.cache.Add(eventID)
}

// PruneCache removes expired entries from the event cache.
// WHY: Called periodically from the main loop to bound memory usage.
func (s *Syncer) PruneCache() {
	s.cache.Prune()
}

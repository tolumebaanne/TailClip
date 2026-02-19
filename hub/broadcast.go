// Author: Toluwalase Mebaanne
// Package main provides a WebSocket broadcaster for real-time clipboard sync.
//
// WHY WebSocket instead of polling:
// Polling has inherent latency (up to one full poll interval) before an agent
// discovers new clipboard content. WebSocket gives us true push delivery:
// the hub sends events the instant they arrive, so paste-on-another-device
// feels instantaneous. It also eliminates wasted HTTP requests when there
// are no new events, reducing network and CPU overhead on both sides.

package main

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tmair/tailclip/shared/models"
)

// Broadcaster manages a set of active WebSocket connections and fans out
// clipboard events to every connected agent in real time.
//
// WHY a dedicated struct:
// Isolates connection lifecycle management (add/remove/broadcast) from HTTP
// routing (server.go) and storage (storage.go). This separation makes it
// easy to test broadcasting without spinning up a full HTTP server.
type Broadcaster struct {
	// mu protects the connections map from concurrent access.
	// WHY a mutex: Go maps are NOT safe for concurrent reads and writes.
	// Multiple goroutines hit AddClient, RemoveClient, and Broadcast
	// simultaneously (one per HTTP/WebSocket handler), so every map
	// access must be serialized to prevent data races and panics.
	mu sync.Mutex

	// connections maps a device ID to its active WebSocket connection.
	// WHY map[string]*websocket.Conn:
	//   - Keyed by device ID so we can quickly look up, replace, or remove
	//     a specific device's connection without iterating the whole set.
	//   - One connection per device: if a device reconnects, the old
	//     connection is replaced, preventing stale duplicate deliveries.
	connections map[string]*websocket.Conn
}

// NewBroadcaster creates a ready-to-use Broadcaster with an empty client map.
// WHY a constructor: Ensures the map is always initialized. A zero-value
// Broadcaster would have a nil map and panic on the first AddClient call.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		connections: make(map[string]*websocket.Conn),
	}
}

// AddClient registers (or replaces) a WebSocket connection for the given device.
//
// WHY replace on duplicate: If an agent reconnects (e.g., after a network
// blip), the hub should seamlessly accept the new connection. Closing the
// old one prevents resource leaks and avoids sending events twice - once on
// the dead connection (which would error) and once on the live one.
func (b *Broadcaster) AddClient(deviceID string, conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close any existing connection for this device before replacing it.
	// WHY: Prevents goroutine leaks and ensures only one active connection
	// per device at any time.
	if existing, ok := b.connections[deviceID]; ok {
		log.Printf("Replacing existing WebSocket for device %s", deviceID)
		existing.Close()
	}

	b.connections[deviceID] = conn
	log.Printf("WebSocket client added: %s (total: %d)", deviceID, len(b.connections))
}

// RemoveClient unregisters a device and closes its WebSocket connection.
//
// WHY explicit cleanup: When an agent disconnects (gracefully or not),
// the hub must remove the stale entry so Broadcast doesn't waste time
// writing to a dead socket. The Close call releases the underlying TCP
// connection, freeing OS-level file descriptors.
func (b *Broadcaster) RemoveClient(deviceID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if conn, ok := b.connections[deviceID]; ok {
		conn.Close()
		delete(b.connections, deviceID)
		log.Printf("WebSocket client removed: %s (total: %d)", deviceID, len(b.connections))
	}
}

// Broadcast sends a clipboard event to every connected agent EXCEPT the one
// that originated the event.
//
// WHY skip the source device:
// If we sent the event back to the originator, the agent would see "new"
// clipboard content, write it to the local clipboard, detect THAT write as
// a change, and push it to the hub again - creating an infinite sync loop.
// Skipping the source breaks this cycle.
//
// WHY continue on error instead of removing:
// A transient write error (e.g., buffer full) doesn't necessarily mean the
// connection is dead. We log the failure but leave cleanup to the read loop
// that monitors connection health. This avoids prematurely dropping clients
// that might recover.
func (b *Broadcaster) Broadcast(event *models.Event, sourceDeviceID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Pre-serialize the event once instead of marshaling per-client.
	// WHY: Avoids redundant JSON encoding when there are many connected
	// devices, reducing CPU usage proportional to client count.
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("ERROR marshaling event for broadcast: %v", err)
		return
	}

	sent := 0
	for deviceID, conn := range b.connections {
		// Skip the device that created this event to prevent sync loops.
		if deviceID == sourceDeviceID {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("ERROR broadcasting to %s: %v", deviceID, err)
			// Don't remove here - let the read-loop handle disconnection.
			// WHY: The read goroutine has better context about whether the
			// connection is truly dead or just temporarily congested.
			continue
		}
		sent++
	}

	if sent > 0 {
		log.Printf("Broadcast event %s to %d client(s) (source: %s)",
			event.EventID, sent, sourceDeviceID)
	}
}

// ClientCount returns the number of currently connected WebSocket clients.
// WHY: Useful for health checks and monitoring - operators can see how many
// agents are actively connected to the hub.
func (b *Broadcaster) ClientCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.connections)
}

// Author: Toluwalase Mebaanne
// Package main provides the HTTP server for the TailClip hub.
//
// WHY a dedicated server file:
// Separates HTTP routing and request handling from storage logic (storage.go).
// This keeps each file focused on one responsibility - the server handles
// network communication while storage handles data persistence.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tmair/tailclip/shared/auth"
	"github.com/tmair/tailclip/shared/models"
)

// Server is the HTTP frontend for the TailClip hub.
// WHY a struct: Holds shared dependencies (storage, config) so handler methods
// can access them without global variables. Makes testing easier since you can
// inject a test Storage instance.
type Server struct {
	storage     *Storage
	broadcaster *Broadcaster
	authToken   string
	mux         *http.ServeMux
}

// NewServer creates a Server wired to the given storage and auth token.
// WHY accept dependencies: Follows dependency injection so callers (main, tests)
// control which storage backend and credentials the server uses.
func NewServer(storage *Storage, broadcaster *Broadcaster, authToken string) *Server {
	s := &Server{
		storage:     storage,
		broadcaster: broadcaster,
		authToken:   authToken,
		mux:         http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

// setupRoutes registers every HTTP endpoint on the internal ServeMux.
// WHY centralized routing: A single place to see the full API surface,
// making it easy to audit endpoints, add middleware, or generate docs later.
func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/api/v1/clipboard/push", s.handlePush)
	s.mux.HandleFunc("/api/v1/history", s.handleHistory)
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/device/register", s.handleRegister)
	s.mux.HandleFunc("/api/v1/ws", s.handleWebSocket)
}

// ServeHTTP delegates to the internal mux so Server satisfies http.Handler.
// WHY implement http.Handler: Lets the server be used directly with
// http.ListenAndServe or wrapped in middleware (logging, CORS, etc.) later.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on the given address.
// WHY a convenience method: Encapsulates the standard http.Server setup with
// sensible timeouts so callers only need to provide an address string.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("Hub listening on %s", addr)
	return srv.ListenAndServe()
}

// --- Handlers ----------------------------------------------------------------

// handlePush receives clipboard events from agents and stores them.
// WHY POST-only: Pushing a clipboard event is a write operation that
// creates a new resource. GET would be semantically wrong and breaks caching.
func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	log.Printf("Push request received from %s", r.RemoteAddr)

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !auth.Authenticate(r, s.authToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Ensure timestamp is set - WHY: Agents might have clock skew, but we
	// still accept their timestamp if present. Only default if missing.
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Compute hash if not provided - WHY: Guarantees deduplication works
	// even if the agent forgot to set the hash before sending.
	if event.TextHash == "" {
		event.SetTextHash()
	}

	if err := s.storage.InsertEvent(&event); err != nil {
		log.Printf("ERROR inserting event: %v", err)
		http.Error(w, "failed to store event", http.StatusInternalServerError)
		return
	}

	log.Printf("Event stored: id=%s source=%s type=%s", event.EventID, event.SourceDeviceID, event.ContentType)

	// Broadcast to all connected WebSocket clients AFTER successful storage.
	// WHY after storage: If storage fails, we don't want to broadcast an event
	// that isn't persisted - agents would receive it but it wouldn't appear in
	// history, causing inconsistency.
	s.broadcaster.Broadcast(&event, event.SourceDeviceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleHistory returns recent clipboard events for agent sync.
// WHY this endpoint exists: Agents poll the hub to discover clipboard events
// from other devices. Without history, a newly started agent would have no
// way to catch up on events it missed while offline.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !auth.Authenticate(r, s.authToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Default to 50 events - WHY: Keeps response size reasonable for routine
	// polling while giving enough history for agents reconnecting after a brief gap.
	limit := 50
	events, err := s.storage.GetRecentEvents(limit)
	if err != nil {
		log.Printf("ERROR fetching history: %v", err)
		http.Error(w, "failed to fetch history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// handleHealth is a lightweight liveness check.
// WHY this endpoint exists: Monitoring tools (uptime checks, load balancers,
// Tailscale health checks) need a fast, unauthenticated endpoint to verify
// the hub process is running and responsive.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRegister allows agents to announce themselves to the hub.
// WHY this endpoint exists: The hub needs to know which devices are in the
// network for health monitoring, event routing, and admin visibility.
// Without registration, the hub has no awareness of connected devices.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !auth.Authenticate(r, s.authToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var device models.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Always update last-seen on registration - WHY: Registration doubles as
	// a heartbeat so the hub knows this device is alive right now.
	device.UpdateLastSeen()

	if err := s.storage.InsertDevice(&device); err != nil {
		log.Printf("ERROR registering device: %v", err)
		http.Error(w, "failed to register device", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "registered",
		"message": fmt.Sprintf("device %s registered", device.DeviceID),
	})
}

// --- WebSocket ---------------------------------------------------------------

// upgrader configures the WebSocket upgrade handshake.
// WHY CheckOrigin returns true: TailClip runs on a private Tailscale network,
// not the public internet. Strict origin checking would block legitimate agent
// connections since they don't come from a browser with an Origin header.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// handleWebSocket upgrades an HTTP connection to WebSocket for real-time
// clipboard event delivery.
//
// WHY this endpoint exists: Agents connect here to receive instant push
// notifications of clipboard events from other devices, instead of polling
// the /api/v1/history endpoint repeatedly.
//
// WHY authenticate via query parameter: WebSocket upgrade requests are standard
// HTTP GETs where custom headers aren't reliably supported across all client
// libraries. The ?token= approach is the widely accepted workaround
// (see shared/auth/token.go for details).
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate using query parameter.
	// WHY query param here: WebSocket clients can't set custom headers during
	// the upgrade handshake, so we fall back to ?token= for auth.
	if !auth.Authenticate(r, s.authToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract device ID from query parameter.
	// WHY required: The broadcaster needs the device ID to register the
	// connection and skip the source device during broadcast.
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id query parameter required", http.StatusBadRequest)
		return
	}

	// Upgrade HTTP connection to WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERROR: WebSocket upgrade failed for device %s: %v", deviceID, err)
		return
	}

	// Register the WebSocket connection with the broadcaster.
	s.broadcaster.AddClient(deviceID, conn)
	log.Printf("WebSocket connected: device=%s", deviceID)

	// Read loop - keeps the connection alive and detects disconnection.
	// WHY a read loop: WebSocket connections require active reading to detect
	// when the remote end disconnects. Without this, the broadcaster would
	// keep trying to write to a dead connection. We don't expect meaningful
	// messages from agents (they push via HTTP), so we just discard reads.
	defer func() {
		s.broadcaster.RemoveClient(deviceID)
		log.Printf("WebSocket disconnected: device=%s", deviceID)
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			// WHY break on error: Any read error (clean close, network drop,
			// etc.) means the connection is done. The deferred RemoveClient
			// will clean up.
			break
		}
	}
}

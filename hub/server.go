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

	"github.com/tmair/tailclip/shared/auth"
	"github.com/tmair/tailclip/shared/models"
)

// Server is the HTTP frontend for the TailClip hub.
// WHY a struct: Holds shared dependencies (storage, config) so handler methods
// can access them without global variables. Makes testing easier since you can
// inject a test Storage instance.
type Server struct {
	storage   *Storage
	authToken string
	mux       *http.ServeMux
}

// NewServer creates a Server wired to the given storage and auth token.
// WHY accept dependencies: Follows dependency injection so callers (main, tests)
// control which storage backend and credentials the server uses.
func NewServer(storage *Storage, authToken string) *Server {
	s := &Server{
		storage:   storage,
		authToken: authToken,
		mux:       http.NewServeMux(),
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

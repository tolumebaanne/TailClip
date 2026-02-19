// Author: Toluwalase Mebaanne
// Package main is the entry point for the TailClip hub server.
//
// WHY a separate main.go:
// Keeps the startup/wiring logic isolated from business logic. server.go owns
// HTTP routing, storage.go owns persistence, broadcast.go owns real-time push,
// and main.go is the thin glue that creates them, connects them, and starts
// listening. This separation means you can test each component independently
// without invoking the full startup sequence.

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tmair/tailclip/shared/config"
)

// defaultConfigPath is the file path checked when no explicit path is given.
// WHY a constant: Makes the default discoverable and easy to change in one
// place if the project's layout conventions evolve.
const defaultConfigPath = "hub-config.json"

func main() {
	// --- Step 1: Load configuration -------------------------------------------
	// WHY load config first: Every other component depends on configuration
	// values (database path, auth token, listen address). If the config is
	// missing or invalid, there's no point initializing anything else - fail
	// fast with a clear error message instead of a cryptic nil-pointer later.
	configPath := defaultConfigPath
	if len(os.Args) > 1 {
		// Allow overriding the config path via command-line argument.
		// WHY: Useful for running multiple hub instances with different configs
		// during development or testing without modifying the default file.
		configPath = os.Args[1]
	}

	cfg, err := config.LoadHubConfig(configPath)
	if err != nil {
		log.Fatalf("FATAL: failed to load hub config from %s: %v", configPath, err)
	}
	log.Printf("Hub config loaded from %s", configPath)

	// --- Step 2: Initialize storage -------------------------------------------
	// WHY storage before server: The server's request handlers need a working
	// database to insert events and query history. Initializing storage first
	// guarantees the schema exists and the database file is writable before
	// we start accepting HTTP traffic.
	storage, err := NewStorage(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("FATAL: failed to initialize storage at %s: %v", cfg.SQLitePath, err)
	}
	// WHY defer Close: Ensures the SQLite WAL is checkpointed and all data is
	// flushed to disk even if the hub exits unexpectedly (e.g., SIGTERM).
	// Without this, the last few writes could be lost.
	defer storage.Close()
	log.Printf("Storage initialized at %s", cfg.SQLitePath)

	// --- Step 3: Create broadcaster -------------------------------------------
	// WHY create broadcaster before server: The server will need a reference
	// to the broadcaster so it can push new clipboard events to connected
	// WebSocket clients immediately after storing them.
	broadcaster := NewBroadcaster()
	log.Printf("Broadcaster initialized")

	// --- Step 4: Create and start server --------------------------------------
	// WHY pass both storage and auth token: Dependency injection keeps the
	// server testable. In tests you can supply a mock storage and a known
	// token without touching config files or environment variables.
	server := NewServer(storage, broadcaster, cfg.AuthToken)

	addr := fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.ListenPort)
	log.Printf("Starting TailClip hub on %s", addr)

	// ListenAndServe blocks until the server encounters a fatal error.
	// WHY log.Fatalf on error: If the listener fails (e.g., port in use,
	// permission denied), there's nothing to recover - exit immediately
	// with a clear message so operators can diagnose the issue.
	if err := server.ListenAndServe(addr); err != nil {
		log.Fatalf("FATAL: hub server failed: %v", err)
	}
}

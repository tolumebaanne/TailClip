// Author: Toluwalase Mebaanne
// Package main provides the hub server's SQLite storage layer.
//
// WHY SQLite:
// SQLite is an embedded database that requires zero external infrastructure.
// For TailClip, this is ideal because:
//   - No separate database server to install, configure, or maintain
//   - Single file storage makes backups trivial (just copy the .db file)
//   - Excellent read performance for clipboard history queries
//   - Write performance is sufficient for clipboard events (not high-frequency)
//   - Ships as part of the hub binary - deploy one file, done
//
// If TailClip ever needs to scale beyond a single hub (unlikely for personal use),
// this layer can be swapped for PostgreSQL by changing the implementation while
// keeping the same method signatures.

package main

import (
	"database/sql"
	"fmt"
	"time"

	// WHY blank import: go-sqlite3 registers itself as a database/sql driver
	// via its init() function. We don't call it directly - database/sql uses it
	// behind the scenes when we Open("sqlite3", ...).
	_ "github.com/mattn/go-sqlite3"

	"github.com/tmair/tailclip/shared/models"
)

// Storage wraps the SQLite database connection and provides methods
// for persisting clipboard events and device registrations.
// WHY a struct: Encapsulates the database connection and provides a clean
// API for the rest of the hub. Makes testing easier (can mock or use in-memory DB).
type Storage struct {
	db *sql.DB
}

// NewStorage initializes the SQLite database and creates tables if they don't exist.
// WHY eager table creation: The hub should be ready to serve immediately after startup.
// Creating tables in NewStorage ensures the schema exists before any requests arrive,
// avoiding race conditions and simplifying error handling in request handlers.
func NewStorage(dbPath string) (*Storage, error) {
	// WHY WAL mode via connection string: Write-Ahead Logging allows concurrent
	// reads while writing, which is important when the hub is receiving new events
	// while agents are polling for history simultaneously.
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify the connection is actually working
	// WHY: sql.Open only validates the driver name, it doesn't connect.
	// Ping forces a real connection attempt so we fail fast on bad paths.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	s := &Storage{db: db}

	if err := s.CreateTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return s, nil
}

// CreateTables sets up the database schema for clipboard events and devices.
// WHY IF NOT EXISTS: Makes the hub idempotent on restart - safe to call multiple
// times without destroying existing data. Critical for a service that may restart
// frequently during development or due to system reboots.
func (s *Storage) CreateTables() error {
	// Events table stores clipboard synchronization history
	// WHY this schema:
	//   - event_id as PRIMARY KEY: natural unique identifier, prevents duplicates
	//   - source_device_id: links to devices table for filtering/routing
	//   - timestamp: indexed for efficient chronological queries (GetRecentEvents)
	//   - content_type: enables type-based filtering as handlers expand
	//   - text: the actual clipboard payload
	//   - text_hash: enables deduplication without full text comparison
	eventsSQL := `
	CREATE TABLE IF NOT EXISTS events (
		event_id        TEXT PRIMARY KEY,
		source_device_id TEXT NOT NULL,
		timestamp       DATETIME NOT NULL,
		content_type    TEXT NOT NULL DEFAULT 'text',
		text            TEXT NOT NULL,
		text_hash       TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_source ON events(source_device_id);
	CREATE INDEX IF NOT EXISTS idx_events_hash ON events(text_hash);
	`

	// Devices table tracks registered agents in the network
	// WHY this schema:
	//   - device_id as PRIMARY KEY: stable unique device identifier
	//   - device_name: human-readable, for UI and logging
	//   - tailscale_ip: network address for direct communication
	//   - last_seen_utc: health monitoring and online status detection
	//   - enabled: administrative control over device participation
	devicesSQL := `
	CREATE TABLE IF NOT EXISTS devices (
		device_id    TEXT PRIMARY KEY,
		device_name  TEXT NOT NULL,
		tailscale_ip TEXT NOT NULL,
		last_seen_utc DATETIME NOT NULL,
		enabled      BOOLEAN NOT NULL DEFAULT 1
	);
	`

	if _, err := s.db.Exec(eventsSQL); err != nil {
		return fmt.Errorf("failed to create events table: %w", err)
	}

	if _, err := s.db.Exec(devicesSQL); err != nil {
		return fmt.Errorf("failed to create devices table: %w", err)
	}

	return nil
}

// InsertEvent stores a new clipboard event in the database.
// WHY INSERT OR IGNORE: If an event with the same event_id already exists
// (e.g., due to agent retry after a network timeout), silently skip it.
// This makes event submission idempotent and safe for unreliable networks.
func (s *Storage) InsertEvent(event *models.Event) error {
	query := `
	INSERT OR IGNORE INTO events (event_id, source_device_id, timestamp, content_type, text, text_hash)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		event.EventID,
		event.SourceDeviceID,
		event.Timestamp.UTC().Format(time.RFC3339),
		event.ContentType,
		event.Text,
		event.TextHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// InsertDevice registers a new device or updates an existing one.
// WHY UPSERT (INSERT OR REPLACE): Devices re-register on startup, and their
// Tailscale IP or name may change. Upsert handles both first registration
// and subsequent updates cleanly without requiring separate insert/update logic.
func (s *Storage) InsertDevice(device *models.Device) error {
	query := `
	INSERT OR REPLACE INTO devices (device_id, device_name, tailscale_ip, last_seen_utc, enabled)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		device.DeviceID,
		device.DeviceName,
		device.TailscaleIP,
		device.LastSeenUTC.UTC().Format(time.RFC3339),
		device.Enabled,
	)
	if err != nil {
		return fmt.Errorf("failed to insert device: %w", err)
	}

	return nil
}

// GetRecentEvents retrieves the most recent clipboard events, ordered newest first.
// WHY limit parameter: Callers control how much history they need. Agents syncing
// for the first time may want more history, while routine polls only need the latest.
// WHY ORDER BY timestamp DESC: Most recent events are most relevant for clipboard sync.
// Agents typically only care about what happened since their last poll.
func (s *Storage) GetRecentEvents(limit int) ([]models.Event, error) {
	query := `
	SELECT event_id, source_device_id, timestamp, content_type, text, text_hash
	FROM events
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		var ts string

		if err := rows.Scan(
			&event.EventID,
			&event.SourceDeviceID,
			&ts,
			&event.ContentType,
			&event.Text,
			&event.TextHash,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		// Parse the stored RFC3339 timestamp back into time.Time
		// WHY: SQLite stores timestamps as text strings. We parse them back
		// to time.Time for consistent handling throughout the application.
		event.Timestamp, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("failed to parse event timestamp: %w", err)
		}

		events = append(events, event)
	}

	// WHY check rows.Err(): The for rows.Next() loop may have exited due to
	// an error (not just running out of rows). This catches iteration errors.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event rows: %w", err)
	}

	return events, nil
}

// Close cleanly shuts down the database connection.
// WHY: Ensures WAL checkpoint completes and all data is flushed to disk.
// Should be called via defer in main() to prevent data loss on shutdown.
func (s *Storage) Close() error {
	return s.db.Close()
}

// Author: Toluwalase Mebaanne
// Package models defines the core data structures for TailClip.
// These models represent the shared state across hub and agent components.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Event represents a single clipboard synchronization event.
// WHY: We need to track each clipboard change across devices in the network.
// This structure captures the what, when, where, and who of clipboard operations.
type Event struct {
	// EventID uniquely identifies this clipboard event
	// WHY: Prevents duplicate processing and enables event ordering/tracking
	EventID string `json:"event_id" db:"event_id"`

	// SourceDeviceID identifies which device created this clipboard event
	// WHY: Essential for preventing sync loops and showing users where content originated
	SourceDeviceID string `json:"source_device_id" db:"source_device_id"`

	// Timestamp records when this event occurred (UTC)
	// WHY: Enables chronological ordering and conflict resolution (last-write-wins)
	Timestamp time.Time `json:"timestamp" db:"timestamp"`

	// ContentType describes the clipboard content format (text, image, file, etc.)
	// WHY: Different content types require different handling and rendering
	ContentType string `json:"content_type" db:"content_type"`

	// Text contains the actual clipboard text content
	// WHY: Stores the payload that needs to be synchronized across devices
	Text string `json:"text" db:"text"`

	// TextHash is a SHA-256 hash of the text content
	// WHY: Enables efficient deduplication without comparing full text content
	// Also useful for privacy (can check if content matches without storing plain text)
	TextHash string `json:"text_hash" db:"text_hash"`
}

// ComputeTextHash generates a SHA-256 hash of the event's text content.
// WHY: Centralized hash computation ensures consistency across the application.
// This is used for deduplication and quick content comparison.
func (e *Event) ComputeTextHash() string {
	hash := sha256.Sum256([]byte(e.Text))
	return hex.EncodeToString(hash[:])
}

// SetTextHash computes and stores the hash of the current text content.
// WHY: Convenience method to ensure the hash is always set when text is updated.
func (e *Event) SetTextHash() {
	e.TextHash = e.ComputeTextHash()
}

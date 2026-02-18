// Author: Toluwalase Mebaanne
// Package models defines the core data structures for TailClip.
// These models represent the shared state across hub and agent components.

package models

import (
	"time"
)

// Device represents a registered device in the TailClip network.
// WHY: We need to track which devices are part of the clipboard sync network,
// monitor their health, and control their participation in synchronization.
type Device struct {
	// DeviceID uniquely identifies this device in the network
	// WHY: Each device needs a stable identifier for routing clipboard events
	// and preventing sync loops (don't send events back to source)
	DeviceID string `json:"device_id" db:"device_id"`

	// DeviceName is a human-readable label for this device
	// WHY: Users need to identify their devices (e.g., "MacBook Pro", "Work Desktop")
	// Makes logs and UI more user-friendly than showing UUIDs or IPs
	DeviceName string `json:"device_name" db:"device_name"`

	// TailscaleIP is the Tailscale VPN IP address for this device
	// WHY: Enables secure peer-to-peer communication over the Tailnet
	// This is how devices directly communicate without exposing public IPs
	TailscaleIP string `json:"tailscale_ip" db:"tailscale_ip"`

	// LastSeenUTC records when this device last checked in with the hub
	// WHY: Allows detection of offline/stale devices for health monitoring
	// Helps UI show device status and debugging connectivity issues
	LastSeenUTC time.Time `json:"last_seen_utc" db:"last_seen_utc"`

	// Enabled controls whether this device participates in clipboard sync
	// WHY: Users may want to temporarily disable sync on specific devices
	// Also useful for administrative control (ban misbehaving devices)
	Enabled bool `json:"enabled" db:"enabled"`
}

// IsOnline checks if the device has been seen recently (within the last 5 minutes).
// WHY: Provides a simple way to determine device health status for UI and routing.
// 5-minute threshold balances responsiveness with tolerance for network hiccups.
func (d *Device) IsOnline() bool {
	threshold := 5 * time.Minute
	return time.Since(d.LastSeenUTC) < threshold
}

// UpdateLastSeen sets the LastSeenUTC to the current time.
// WHY: Convenience method for heartbeat/ping handlers to update device status.
func (d *Device) UpdateLastSeen() {
	d.LastSeenUTC = time.Now().UTC()
}

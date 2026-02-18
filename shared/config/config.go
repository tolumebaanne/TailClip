// Author: Toluwalase Mebaanne
// Package config provides configuration management for both hub and agent components.
// WHY: Centralizes configuration loading, validation, and environment variable handling
// to ensure consistent behavior across the TailClip system.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// HubConfig defines the configuration for the TailClip hub server.
// WHY: The hub is the central coordinator that stores clipboard history and
// synchronizes events across devices. It needs its own configuration separate
// from agents to control server behavior, storage, and security.
type HubConfig struct {
	// ListenIP is the IP address the hub HTTP server binds to
	// WHY: Controls network interface binding (0.0.0.0 for all, 127.0.0.1 for local,
	// or specific Tailscale IP for Tailnet-only access)
	ListenIP string `json:"listen_ip"`

	// ListenPort is the TCP port the hub HTTP server listens on
	// WHY: Allows customization to avoid port conflicts with other services
	ListenPort int `json:"listen_port"`

	// AuthToken is the shared secret for authenticating agent requests
	// WHY: Prevents unauthorized devices from accessing clipboard data or
	// injecting malicious events into the sync network
	AuthToken string `json:"auth_token"`

	// SQLitePath is the file path to the SQLite database
	// WHY: Clipboard events and device registrations need persistent storage
	// SQLite provides a simple, embedded database without external dependencies
	SQLitePath string `json:"sqlite_path"`

	// HistoryLimit is the maximum number of clipboard events to retain
	// WHY: Prevents unbounded database growth while keeping recent history
	// accessible for syncing new devices or recovering lost clipboard items
	HistoryLimit int `json:"history_limit"`

	// RetentionDays is how many days to keep clipboard history before deletion
	// WHY: Privacy and storage management - old clipboard data should be purged
	// to protect user privacy and prevent storage bloat
	RetentionDays int `json:"retention_days"`
}

// AgentConfig defines the configuration for a TailClip agent (client device).
// WHY: Each device running the agent needs to know how to connect to the hub,
// identify itself, and control its sync behavior independently.
type AgentConfig struct {
	// DeviceID is a unique identifier for this agent device
	// WHY: Differentiates this device from others in the network and prevents
	// sync loops (agents won't apply their own clipboard events)
	DeviceID string `json:"device_id"`

	// DeviceName is a human-readable name for this device
	// WHY: Makes logs and UI more user-friendly (e.g., "MacBook Pro" vs UUID)
	DeviceName string `json:"device_name"`

	// HubURL is the full URL to the TailClip hub server
	// WHY: Agents need to know where to send clipboard events and poll for updates
	// Typically a Tailscale IP like http://100.64.0.1:8080
	HubURL string `json:"hub_url"`

	// AuthToken is the shared secret for authenticating with the hub
	// WHY: Must match the hub's auth_token to prove this is an authorized device
	AuthToken string `json:"auth_token"`

	// Enabled controls whether this agent actively syncs clipboard
	// WHY: Users may want to temporarily disable sync without uninstalling
	// (e.g., during sensitive work or when troubleshooting)
	Enabled bool `json:"enabled"`

	// PollIntervalMs is how often (in milliseconds) to check hub for new events
	// WHY: Balances sync responsiveness vs network/CPU overhead
	// Lower = faster sync but more resource usage
	PollIntervalMs int `json:"poll_interval_ms"`

	// NotifyEnabled controls whether to show desktop notifications for synced clips
	// WHY: Some users want silent sync, others want visual confirmation
	// of clipboard updates from other devices
	NotifyEnabled bool `json:"notify_enabled"`
}

// LoadHubConfig reads hub configuration from a JSON file with environment variable fallbacks.
// WHY: Configuration should be flexible - load from file for persistence, but allow
// environment variables to override sensitive values (e.g., in Docker/containers).
func LoadHubConfig(path string) (*HubConfig, error) {
	config := &HubConfig{
		// Default values - WHY: Provide sensible defaults for optional settings
		ListenIP:      "0.0.0.0",
		ListenPort:    8080,
		SQLitePath:    "tailclip.db",
		HistoryLimit:  1000,
		RetentionDays: 30,
	}

	// Read configuration file if it exists
	// WHY: File-based config is easier to manage than environment variables for
	// non-sensitive settings and provides a single source of truth
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse hub config: %w", err)
		}
	}

	// Environment variable overrides for sensitive values
	// WHY: Never commit secrets to config files - allow injection via env vars
	// for secure deployment (containers, CI/CD, cloud platforms)
	if token := os.Getenv("TAILCLIP_HUB_AUTH_TOKEN"); token != "" {
		config.AuthToken = token
	}

	if port := os.Getenv("TAILCLIP_HUB_PORT"); port != "" {
		var portNum int
		if _, err := fmt.Sscanf(port, "%d", &portNum); err == nil {
			config.ListenPort = portNum
		}
	}

	// Validation - WHY: Fail fast with clear errors rather than starting with invalid config
	if config.AuthToken == "" {
		return nil, fmt.Errorf("auth_token is required (set in config file or TAILCLIP_HUB_AUTH_TOKEN env var)")
	}

	return config, nil
}

// LoadAgentConfig reads agent configuration from a JSON file with environment variable fallbacks.
// WHY: Same rationale as LoadHubConfig - file for persistence, env vars for sensitive overrides.
func LoadAgentConfig(path string) (*AgentConfig, error) {
	config := &AgentConfig{
		// Default values - WHY: Reasonable defaults for a responsive sync experience
		Enabled:        true,
		PollIntervalMs: 1000, // 1 second polling
		NotifyEnabled:  true,
	}

	// Read configuration file if it exists
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse agent config: %w", err)
		}
	}

	// Environment variable overrides
	// WHY: Auth token should NEVER be hardcoded - always inject securely
	if token := os.Getenv("TAILCLIP_AGENT_AUTH_TOKEN"); token != "" {
		config.AuthToken = token
	}

	if hubURL := os.Getenv("TAILCLIP_HUB_URL"); hubURL != "" {
		config.HubURL = hubURL
	}

	if deviceID := os.Getenv("TAILCLIP_DEVICE_ID"); deviceID != "" {
		config.DeviceID = deviceID
	}

	// Validation - WHY: Agents can't function without knowing their identity and hub location
	if config.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required (set in config file or TAILCLIP_DEVICE_ID env var)")
	}

	if config.DeviceName == "" {
		return nil, fmt.Errorf("device_name is required (set in config file)")
	}

	if config.HubURL == "" {
		return nil, fmt.Errorf("hub_url is required (set in config file or TAILCLIP_HUB_URL env var)")
	}

	if config.AuthToken == "" {
		return nil, fmt.Errorf("auth_token is required (set in config file or TAILCLIP_AGENT_AUTH_TOKEN env var)")
	}

	return config, nil
}

// GetPollInterval returns the agent's poll interval as a time.Duration.
// WHY: Convenience method to convert milliseconds to Go's standard duration type
// for use with time.Ticker and other timing operations.
func (c *AgentConfig) GetPollInterval() time.Duration {
	return time.Duration(c.PollIntervalMs) * time.Millisecond
}

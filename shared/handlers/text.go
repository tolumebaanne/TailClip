// Author: Toluwalase Mebaanne
// TextHandler implements the ContentHandler interface for plain text clipboard content.
//
// WHY this is Phase 1:
// Text is the most common clipboard content type and the simplest to implement.
// Starting here lets us validate the entire sync pipeline end-to-end (copy on
// Device A → hub → paste on Device B) before tackling complex types like images
// or files that require binary transport, chunking, and format negotiation.
//
// FUTURE EXPANSION:
// Phase 2 - ImageHandler: will handle image/png, image/jpeg clipboard content
// Phase 3 - FileHandler: will handle file references and small file transfers
// Each new handler follows this same pattern: implement ContentHandler, register it.

package handlers

import (
	"fmt"
	"strings"
)

// MaxTextLength is the maximum allowed text content length in bytes.
// WHY: Prevents abuse and memory issues from extremely large clipboard contents.
// 1MB is generous for text while protecting against accidental binary pastes.
const MaxTextLength = 1 * 1024 * 1024 // 1 MB

// TextHandler processes plain text clipboard content.
// WHY a struct instead of bare functions:
// Struct-based handlers can carry configuration (e.g., max length, encoding)
// and satisfy the ContentHandler interface cleanly. This also allows
// dependency injection for testing.
type TextHandler struct{}

// NewTextHandler creates a new TextHandler instance.
// WHY a constructor: Provides a consistent creation pattern across all handlers.
// As handlers grow to accept configuration, the constructor is where defaults
// and validation will live.
func NewTextHandler() *TextHandler {
	return &TextHandler{}
}

// CanHandle returns true if the content type is plain text.
// WHY case-insensitive comparison: Content-Type headers from different OS
// clipboard APIs may vary in casing. Normalizing prevents missed matches.
func (h *TextHandler) CanHandle(contentType string) bool {
	return strings.EqualFold(contentType, "text")
}

// Process validates and sanitizes plain text clipboard content.
// WHY: Even simple text needs validation before storage and sync:
//   - Empty content wastes sync bandwidth and storage
//   - Oversized content could exhaust memory or database limits
//   - Future: encoding normalization, content filtering, etc.
func (h *TextHandler) Process(content string) error {
	// Reject empty content
	// WHY: Syncing empty strings is pointless and likely indicates
	// a clipboard clear event, not actual content to share
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("text content is empty")
	}

	// Enforce size limit
	// WHY: Protects the hub from memory pressure and ensures SQLite
	// rows stay within reasonable bounds for query performance
	if len(content) > MaxTextLength {
		return fmt.Errorf("text content exceeds maximum length of %d bytes", MaxTextLength)
	}

	return nil
}

// GetType returns the content type identifier for this handler.
// WHY: Used by the handler registry, logging, and metrics to identify
// which handler processed a clipboard event.
func (h *TextHandler) GetType() string {
	return "text"
}

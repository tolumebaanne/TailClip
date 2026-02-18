// Author: Toluwalase Mebaanne
// Package handlers defines a pluggable content handling system for TailClip.
// WHY: Clipboard content comes in many forms - text, images, files, rich text, etc.
// Rather than baking all content type logic into a single monolithic function,
// we define an interface that each content type implements independently.
// This follows the Strategy Pattern and Open/Closed Principle:
//   - Open for extension: add new content types by implementing the interface
//   - Closed for modification: existing handlers don't change when new types arrive
//
// ARCHITECTURE BENEFIT:
// When we add image sync (Phase 2) or file sync (Phase 3), we simply create
// ImageHandler and FileHandler structs that implement ContentHandler.
// No existing code needs to be touched - just register the new handler.

package handlers

// ContentHandler defines the contract for processing clipboard content types.
//
// WHY an interface instead of a switch statement:
// A switch/case on content type works initially, but becomes unmaintainable as
// types grow. Each new type adds another case, leading to a bloated function.
// An interface isolates each type's logic in its own file, making the codebase
// modular and testable. Each handler can be developed, tested, and reviewed
// independently.
//
// WHY these specific methods:
//   - CanHandle: enables dynamic dispatch - a registry of handlers can find the
//     right one for incoming content without hardcoded type mappings
//   - Process: performs the actual content handling (validation, transformation,
//     storage preparation) - each type has unique processing needs
//   - GetType: identifies the handler for logging, metrics, and debugging
type ContentHandler interface {
	// CanHandle returns true if this handler supports the given content type.
	// WHY: Enables a handler registry to dynamically route content to the
	// correct handler without maintaining a separate mapping table.
	CanHandle(contentType string) bool

	// Process performs content-specific handling (validation, sanitization, etc).
	// WHY: Different content types need different processing - text may need
	// encoding normalization, images need format validation, files need size checks.
	Process(content string) error

	// GetType returns the content type string this handler is responsible for.
	// WHY: Useful for logging, metrics collection, and debugging which handler
	// processed a given clipboard event.
	GetType() string
}

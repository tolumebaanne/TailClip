// Author: Toluwalase Mebaanne
// Package main provides clipboard read/write operations for the TailClip agent.
//
// WHY a cross-platform clipboard library (github.com/atotto/clipboard):
// Clipboard access is deeply OS-specific: macOS uses pbcopy/pbpaste, Linux uses
// xclip/xsel (X11) or wl-copy/wl-paste (Wayland), and Windows uses Win32 APIs.
// Writing and maintaining native implementations for each OS would be a large,
// error-prone effort. atotto/clipboard abstracts this behind a simple Read/Write
// interface, letting TailClip support all three platforms with zero OS-specific code.
//
// WHY polling instead of event-driven clipboard monitoring:
// Most operating systems do not provide a reliable, cross-platform clipboard
// change notification API. macOS has NSPasteboard.changeCount, Windows has
// AddClipboardFormatListener, but Linux/Wayland has none. Rather than maintain
// three separate event-driven implementations (and handle their edge cases),
// polling at a configurable interval provides uniform behavior everywhere.
// The trade-off is a small latency (up to one poll interval) before detecting
// changes, which is acceptable for clipboard sync (humans don't paste
// faster than ~1 second apart).

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/atotto/clipboard"
)

// ReadClipboard returns the current clipboard text content.
//
// WHY return empty string on error instead of propagating:
// Clipboard read failures are transient (e.g., clipboard locked by another app,
// clipboard empty). The polling loop treats an empty return the same as "no
// change", so there's no need to bubble up the error and complicate the caller.
// We log the error for debugging but don't crash or stop polling.
func ReadClipboard() string {
	text, err := clipboard.ReadAll()
	if err != nil {
		// WHY log instead of return error: Clipboard errors are frequent and
		// usually harmless (empty clipboard, app holding lock). Logging keeps
		// visibility without disrupting the sync loop.
		log.Printf("WARN: failed to read clipboard: %v", err)
		return ""
	}
	return text
}

// WriteClipboard sets the system clipboard to the given text.
//
// WHY return error here (unlike ReadClipboard):
// A failed write means the user won't see synced content - that's a visible
// problem worth reporting to the caller so it can decide how to handle it
// (retry, notify user, etc.). Read failures are invisible; write failures are not.
func WriteClipboard(text string) error {
	if err := clipboard.WriteAll(text); err != nil {
		log.Printf("ERROR: failed to write clipboard: %v", err)
		return err
	}
	return nil
}

// GetClipboardHash reads the current clipboard and returns its SHA-256 hash.
//
// WHY hash-based change detection instead of comparing full text:
//   - Memory efficiency: Clipboard can hold megabytes of text. Storing and
//     comparing a 64-char hex hash is far cheaper than keeping a full copy
//     of the last clipboard content in memory.
//   - Consistency: Uses the same hashing approach as models.Event.TextHash,
//     so the agent can compare local clipboard state against hub events
//     by hash alone, avoiding redundant content transfer.
//   - Privacy-friendly: Hashes can be logged or transmitted for debugging
//     without exposing actual clipboard content.
func GetClipboardHash() string {
	text := ReadClipboard()
	if text == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

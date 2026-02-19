// Author: Toluwalase Mebaanne
// Package main provides desktop notification support for the TailClip agent.
//
// WHY desktop notifications for clipboard sync:
// Clipboard sync happens silently in the background - the user has no way to
// know that their clipboard just changed due to a sync event (as opposed to
// their own copy action). Without notifications, unexpected clipboard content
// can be confusing ("I didn't copy this?") or even a security concern.
// A brief notification like "Clipboard synced from MacBook Pro: meeting notes..."
// gives the user awareness and confidence in the system.
//
// WHY optional (config-controlled):
// Not all users want visual interruptions. Power users who understand the sync
// flow may prefer silent operation. Making notifications configurable respects
// different work styles and accessibility needs. The config.NotifyEnabled flag
// is checked by the caller (main.go/sync.go), not here, to keep this package
// focused on the notification itself.
//
// WHY github.com/gen2brain/beeep:
// Native notification APIs are OS-specific (NSUserNotificationCenter on macOS,
// libnotify/D-Bus on Linux, WinToast on Windows). beeep provides a single
// cross-platform Go API that maps to the correct native mechanism on each OS,
// keeping TailClip's codebase free of build tags and platform-specific code.

package main

import (
	"log"

	"github.com/gen2brain/beeep"
)

// appName is the title shown in notification popups.
// WHY a constant: Ensures consistent branding across all notifications
// and provides a single place to update the app name if it changes.
const appName = "TailClip"

// ShowNotification displays a desktop notification when clipboard content
// arrives from another device.
//
// WHY accept sourceDevice and textPreview as parameters:
// The caller (ReceiveFromHub in sync.go) controls what information is shown.
// This function doesn't need to know about Event structs or config - it just
// displays the formatted message. This separation keeps notifications testable
// and decoupled from sync logic.
//
// WHY truncation is the caller's responsibility:
// Different callers might want different preview lengths (e.g., notifications
// vs. log messages). Keeping truncation in the caller gives maximum flexibility.
// sync.go truncates to 80 chars before calling this function.
//
// WHY log errors but don't return them:
// Notification failures are non-critical - the clipboard sync still worked.
// Crashing or complicating the caller's error handling for a failed toast
// notification would be disproportionate. We log for debugging and move on.
func ShowNotification(sourceDevice, textPreview string) {
	title := appName + " - Clipboard Synced"
	body := "From " + sourceDevice + ":\n" + textPreview

	// beeep.Notify sends a native desktop notification.
	// WHY empty string for icon path: Uses the system default notification
	// icon. We can add a custom TailClip icon later without changing the API.
	if err := beeep.Notify(title, body, ""); err != nil {
		// WHY log instead of propagate: Notification failure should never
		// interrupt clipboard sync. The sync itself already succeeded by
		// the time we get here.
		log.Printf("WARN: failed to show notification: %v", err)
	}
}

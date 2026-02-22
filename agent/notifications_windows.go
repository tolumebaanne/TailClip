//go:build windows
package main

import (
	"log"

	"gopkg.in/toast.v1"
)

// ShowNotification displays a desktop notification when clipboard content
// arrives from another device.
func ShowNotification(sourceDevice, textPreview string) {
	title := "TailClip - Clipboard Synced"
	body := "From " + sourceDevice + ":\n" + textPreview

	notification := toast.Notification{
		AppID:   "TailClip",
		Title:   title,
		Message: body,
		Icon:    "",
		Actions: nil,
	}

	if err := notification.Push(); err != nil {
		log.Printf("WARN: failed to show notification: %v", err)
	}
}

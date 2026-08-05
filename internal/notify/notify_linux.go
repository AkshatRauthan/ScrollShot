//go:build linux

// Package notify sends best-effort desktop notifications after a successful
// save. Errors are always silently ignored — notifications are never critical
// to the tool's function.
package notify

import "os/exec"

// Send shows a desktop notification with title and body.
// Uses notify-send (libnotify), which is available on GNOME, KDE, Xfce,
// and most other Linux desktops out of the box.
func Send(title, body string) {
	defer func() { recover() }()
	exec.Command("notify-send", title, body).Run()
}

//go:build !linux && !windows

// Package notify sends best-effort desktop notifications after a successful
// save. Errors are always silently ignored — notifications are never critical
// to the tool's function.
package notify

// Send is a no-op on platforms where no notification backend is
// implemented yet (macOS, BSD, etc.). Scrollshot still prints "Saved: <path>"
// to stdout, so the user always knows where the file went.
func Send(title, body string) {}

// Package capture defines the pluggable interface for taking a screenshot
// of the active window, plus a registry so main.go can pick a backend
// without knowing which OS/compositor it's running on.
//
// Today there is one backend (gnome.go, which shells out to
// gnome-screenshot). Future backends — a direct D-Bus portal client for
// any Wayland desktop, an X11 backend, a Windows backend — implement the
// same Capturer interface and register themselves the same way, so
// nothing outside this package needs to change when they're added.
package capture

import "image"

// Capturer captures the currently active/focused window and returns it
// as a decoded image, ready for the stitcher to consume.
type Capturer interface {
	// Name identifies the backend, e.g. "gnome-screenshot", "portal", "x11".
	Name() string

	// Available reports whether this backend can actually run in the
	// current environment (required binary installed, correct session
	// type, etc). Used to pick a backend automatically.
	Available() bool

	// CaptureActiveWindow captures the focused window and returns it.
	CaptureActiveWindow() (image.Image, error)
}

// registry holds every backend that has registered itself via Register.
// Order matters: Detect() returns the first Available() one, so backends
// should register in priority order (most specific/reliable first).
var registry []Capturer

// Register adds a backend to the registry. Backends call this from an
// init() function in their own file, so simply importing a backend's
// package is enough to make it available for selection.
func Register(c Capturer) {
	registry = append(registry, c)
}

// Detect returns the first registered backend that reports itself as
// Available in the current environment. Returns nil if none match.
func Detect() Capturer {
	for _, c := range registry {
		if c.Available() {
			return c
		}
	}
	return nil
}

// Get returns a specific backend by name, or nil if it isn't registered.
// Lets a user force a backend via a flag/config instead of auto-detecting.
func Get(name string) Capturer {
	for _, c := range registry {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// List returns the names of every registered backend, for error messages
// and a future `scrollshot backends` debug command.
func List() []string {
	names := make([]string, len(registry))
	for i, c := range registry {
		names[i] = c.Name()
	}
	return names
}

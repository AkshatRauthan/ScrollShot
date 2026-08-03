// Package autoscroll drives scrolling (internal/capture only captures
// screenshots) and provides the infrastructure for automated scrolling
// and capture.
//
// This mirrors internal/capture's Scroller-per-backend registry pattern
// on purpose: scroll simulation is just as OS/compositor-restricted as
// screenshot capture is (e.g. Wayland's input-injection protocols are
// locked down similarly to its screenshot protocols), so the same
// "register available backends, auto-detect one" shape applies here too.

package autoscroll

import "errors"

// ErrNotImplemented is returned by every backend registered in this
// package until they're built out.
var ErrNotImplemented = errors.New("autoscroll: not yet implemented")

// Scroller drives scrolling in the currently focused window.
type Scroller interface {
	// Name identifies the backend, e.g. "x11-xtest", "ydotool".
	Name() string

	// Available reports whether this backend can run in the current
	// environment.
	Available() bool

	// ScrollDown requests that the focused window scroll downward by approximately amountPx pixels.
	// The implementation is best-effort; exact pixel movement isn't required.
	ScrollDown(amountPx int) error
}

// registry contains every backend registered via init().
var registry []Scroller

// Register adds a backend to the registry. Backends call this from an
// init() function in their own (build-tagged) file.
func Register(s Scroller) {
	registry = append(registry, s)
}

// Detect returns the first registered backend that reports itself as
// Available in the current environment. Returns nil if none match.
func Detect() Scroller {
	for _, s := range registry {
		if s.Available() {
			return s
		}
	}
	return nil
}

// Get returns the backend with the given name, or nil if it isn't registered.
func Get(name string) Scroller {
	for _, s := range registry {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

// List returns the names of all registered backends.
func List() []string {
	names := make([]string, 0, len(registry))
	for _, s := range registry {
		names = append(names, s.Name())
	}
	return names
}

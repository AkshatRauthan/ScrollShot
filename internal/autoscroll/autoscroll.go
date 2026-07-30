// Package autoscroll drives the scrolling itself (as opposed to
// internal/capture, which only takes the screenshot) — for the planned
// "autoscroll + auto-capture until screen end or user stoppage" feature.
//
// This mirrors internal/capture's Scroller-per-backend registry pattern
// on purpose: scroll simulation is just as OS/compositor-restricted as
// screenshot capture is (e.g. Wayland's input-injection protocols are
// locked down similarly to its screenshot protocols), so the same
// "register available backends, auto-detect one" shape applies here too.
//
// Not yet implemented — this file locks in the contract for when that
// feature gets built.
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

	// ScrollDown scrolls the focused window down by roughly amountPx
	// pixels (backends may only support "scroll by N wheel clicks" and
	// approximate this).
	ScrollDown(amountPx int) error

	// AtBottom attempts to detect whether the window has reached the end
	// of its scrollable content. Best-effort — depends on the backend;
	// callers should also support manual stop as a fallback regardless.
	AtBottom() (bool, error)
}

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

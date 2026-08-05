// Package autoscroll provides the high-level API for automatic scrolling.
// Platform-specific implementations live under the backend package and are
// intentionally hidden from callers.
package autoscroll

import (
	"errors"
	"fmt"

	"scrollshot/internal/autoscroll/backend"
)

var (
	// ErrNoBackend is returned when no supported scrolling backend is
	// available on the current platform.
	ErrNoBackend = errors.New("autoscroll: no supported scrolling backend available")
)

// DetectScroller discovers and returns the best scrolling backend available
// on the current system. The actual selection is made by the build-tag-gated
// backend.Detect() function — one per platform — so this function is fully
// platform-agnostic.
func DetectScroller() (backend.Scroller, error) {
	scroller, err := backend.Detect()
	if err != nil {
		return nil, fmt.Errorf("detecting backend: %w", err)
	}

	if scroller == nil {
		return nil, ErrNoBackend
	}

	if !scroller.Available() {
		return nil, ErrNoBackend
	}

	return scroller, nil
}

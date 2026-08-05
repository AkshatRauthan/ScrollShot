// Package backend contains platform-specific autoscroll implementations.
package backend

// Scroller scrolls the currently focused window.
//
// Implementations are platform-specific (Linux, Windows, macOS, etc.)
// but expose a common interface to the autoscroll controller.
type Scroller interface {
	// Name identifies the backend implementation.
	Name() string

	// Available reports whether this backend can run in the current
	// environment.
	//
	// Implementations should verify any required runtime dependencies
	// (external binaries, libraries, permissions, etc.).
	Available() bool

	// ScrollDown scrolls the currently focused window by approximately
	// amountPx pixels.
	//
	// Implementations are best-effort. Exact pixel movement is neither
	// required nor generally possible across desktop environments.
	ScrollDown(amountPx int) error

	// Close releases any platform resources held by the backend
	// (virtual devices, file descriptors, etc.).
	Close() error
}

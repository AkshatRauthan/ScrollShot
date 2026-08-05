//go:build linux

package backend

// Detect returns the preferred scrolling backend for Linux.
// Called by autoscroll.DetectScroller; platform-specific implementations
// live in the sibling files (windows.go, etc.).
func Detect() (Scroller, error) {
	return NewLinuxUInput()
}

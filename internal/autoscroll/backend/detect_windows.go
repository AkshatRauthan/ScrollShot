//go:build windows

package backend

// Detect returns the preferred scrolling backend for Windows.
// Called by autoscroll.DetectScroller; platform-specific implementations
// live in the sibling files (linux_uinput.go, etc.).
func Detect() (Scroller, error) {
	return NewWindowsSendInput()
}

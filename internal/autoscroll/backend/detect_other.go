//go:build !linux && !windows

package backend

import "errors"

// Detect returns an error on platforms where no scrolling backend has been
// implemented yet (macOS, BSD, etc.).
func Detect() (Scroller, error) {
	return nil, errors.New("autoscroll: no scrolling backend available for this platform")
}

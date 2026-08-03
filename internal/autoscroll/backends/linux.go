package backends

import (
	"errors"
)

// Scroller provides Linux scrolling.
//
// Stage 1 is intentionally a stub.
// Actual X11/Wayland scrolling will be implemented next.
type Scroller struct{}

func New() *Scroller {
	return &Scroller{}
}

func (s *Scroller) Name() string {
	return "linux"
}

func (s *Scroller) Available() bool {
	return true
}

func (s *Scroller) ScrollDown(amountPx int) error {

	if amountPx <= 0 {
		return errors.New("scroll amount must be positive")
	}

	// TODO(v0.3.0):
	// Translate pixel distance into native input events.
	//
	// X11:
	//   XTestFakeButtonEvent()
	//
	// Wayland:
	//   compositor-specific implementation.
	//
	// This stub allows the controller to be developed independently
	// from the platform backend.

	return nil
}

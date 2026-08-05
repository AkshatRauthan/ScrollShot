//go:build linux

package backend

import (
	"errors"
	"fmt"

	"github.com/bendahl/uinput"
)

const (
	uinputDevice = "/dev/uinput"

	// Approximate conversion from pixels to wheel notches.
	pixelsPerWheelNotch = 120
)

// LinuxUInput scrolls by emitting native Linux mouse wheel events
// through a virtual uinput mouse device.
type LinuxUInput struct {
	mouse uinput.Mouse
}

// New returns the preferred Linux scrolling backend.
func NewLinuxUInput() (*LinuxUInput, error) {
	mouse, err := uinput.CreateMouse(
		uinputDevice,
		[]byte("scrollshot"),
	)
	if err != nil {
		return nil, fmt.Errorf("create uinput mouse: %w", err)
	}

	return &LinuxUInput{
		mouse: mouse,
	}, nil
}

func (s *LinuxUInput) Name() string {
	return "linux-uinput"
}

func (s *LinuxUInput) Available() bool {
	return s.mouse != nil
}

func (s *LinuxUInput) ScrollDown(amountPx int) error {
	if s.mouse == nil {
		return errors.New("uinput mouse not initialized")
	}

	if amountPx <= 0 {
		return errors.New("scroll amount must be positive")
	}

	notches := amountPx / pixelsPerWheelNotch
	if notches < 1 {
		notches = 1
	}

	for i := 0; i < notches; i++ {
		if err := s.mouse.Wheel(false, -1); err != nil {
			return fmt.Errorf("emit wheel event: %w", err)
		}
	}

	return nil
}

func (s *LinuxUInput) Close() error {
	if s.mouse == nil {
		return nil
	}

	return s.mouse.Close()
}

//go:build linux

package backend

import (
	"errors"
	"fmt"

	"github.com/bendahl/uinput"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgbutil"
	"github.com/jezek/xgbutil/ewmh"
)

const (
	uinputDevice = "/dev/uinput"

	// Approximate conversion from pixels to wheel notches.
	pixelsPerWheelNotch = 120
)

// LinuxUInput scrolls by emitting native Linux mouse wheel events
// through a virtual uinput mouse device.
//
// Before each scroll it attempts to warp the X11 cursor to the centre of
// the active window (the same step the Windows backend performs via
// SetCursorPos). On X11 this ensures wheel events land on the correct
// window regardless of where the user left the physical cursor; on
// Wayland the X11 connection attempt fails gracefully and the step is
// silently skipped — uinput events are still delivered to the focused
// surface by the compositor.
type LinuxUInput struct {
	mouse uinput.Mouse

	// xu holds an X11 connection for cursor centering.
	// nil when running under Wayland or when no X display is reachable.
	xu *xgbutil.XUtil
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

	// Best-effort X11 connection for cursor centering.
	// Silently ignored on Wayland (DISPLAY not set) or when the X server
	// is not reachable — scroll still works, just without cursor warping.
	xu, _ := xgbutil.NewConn()

	return &LinuxUInput{
		mouse: mouse,
		xu:    xu,
	}, nil
}

func (s *LinuxUInput) Name() string { return "linux-uinput" }

func (s *LinuxUInput) Available() bool { return s.mouse != nil }

// ScrollDown scrolls the currently focused window downward by approximately
// amountPx pixels.
//
// On X11 sessions the cursor is first warped to the centre of the active
// window so the wheel events are routed to the correct application. On
// Wayland the warp is skipped (the compositor routes events by surface
// focus, not cursor position).
func (s *LinuxUInput) ScrollDown(amountPx int) error {
	if s.mouse == nil {
		return errors.New("uinput mouse not initialized")
	}
	if amountPx <= 0 {
		return errors.New("scroll amount must be positive")
	}

	// Move the cursor to the active window's centre on X11 so wheel
	// events are delivered to the correct window.
	if s.xu != nil {
		s.centerCursorOnActiveWindow()
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

// centerCursorOnActiveWindow warps the X11 cursor to the centre of the
// currently active window. It is purely best-effort: any failure (window
// not found, geometry unavailable, translate error) is silently ignored so
// that scroll still proceeds even if the warp cannot be completed.
func (s *LinuxUInput) centerCursorOnActiveWindow() {
	win, err := ewmh.ActiveWindowGet(s.xu)
	if err != nil || win == 0 {
		return
	}

	// GetGeometry gives us width/height (in window-local coordinates).
	geom, err := xproto.GetGeometry(s.xu.Conn(), xproto.Drawable(win)).Reply()
	if err != nil {
		return
	}

	// TranslateCoordinates converts the window's top-left corner (0,0)
	// into root/screen-absolute coordinates.
	coords, err := xproto.TranslateCoordinates(
		s.xu.Conn(), win, s.xu.RootWin(), 0, 0,
	).Reply()
	if err != nil {
		return
	}

	cx := int16(int(coords.DstX) + int(geom.Width)/2)
	cy := int16(int(coords.DstY) + int(geom.Height)/2)

	// WarpPointer with src=None moves the cursor to an absolute position
	// on the root window regardless of where it currently is.
	xproto.WarpPointer(
		s.xu.Conn(),
		xproto.WindowNone, s.xu.RootWin(),
		0, 0, 0, 0,
		cx, cy,
	)
}

func (s *LinuxUInput) Close() error {
	if s.xu != nil {
		s.xu.Conn().Close()
		s.xu = nil
	}
	if s.mouse == nil {
		return nil
	}
	return s.mouse.Close()
}

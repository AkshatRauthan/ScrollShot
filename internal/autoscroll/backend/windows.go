//go:build windows

package backend

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	inputMouse       = 0
	mouseeventfWheel = 0x0800

	// wheelDelta is the Win32 WHEEL_DELTA constant — one standard scroll
	// "click" expressed in wheel units. Negative = scroll down (toward user).
	wheelDelta = 120

	// pixelsPerWheelNotch is the approximate number of pixels scrolled per
	// wheel click. Applications interpret WHEEL_DELTA themselves; this value
	// is a calibrated midpoint across browsers and code editors.
	pixelsPerWheelNotch = 120
)

var (
	winUser32        = syscall.NewLazyDLL("user32.dll")
	procGetFgWindow  = winUser32.NewProc("GetForegroundWindow")
	procGetWinRect   = winUser32.NewProc("GetWindowRect")
	procSetCursorPos = winUser32.NewProc("SetCursorPos")
	procSendInput    = winUser32.NewProc("SendInput")
)

// winRECT mirrors the Win32 RECT structure.
type winRECT struct {
	left, top, right, bottom int32
}

// mouseInput mirrors the Win32 MOUSEINPUT structure.
//
// The explicit blank field before dwExtraInfo is required to reproduce the
// C struct's memory layout on 64-bit Windows: ULONG_PTR demands 8-byte
// alignment, so the compiler inserts 4 bytes of padding after `time`.
// Getting this wrong causes SendInput to silently drop all events.
type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	_           uint32  // padding: aligns dwExtraInfo to 8-byte boundary
	dwExtraInfo uintptr // 8 bytes on 64-bit Windows
}

// inputRecord mirrors the Win32 INPUT structure.
//
// On 64-bit Windows the anonymous union is 8-byte aligned, so there are
// 4 bytes of padding between inputType and the union body. We embed only
// the MOUSEINPUT variant because wheel scrolling is all we ever emit.
//
// Total size: 4 (inputType) + 4 (pad) + 32 (mouseInput) = 40 bytes,
// matching sizeof(INPUT) on 64-bit Windows.
type inputRecord struct {
	inputType uint32
	_         uint32 // padding: aligns union to 8-byte boundary
	mi        mouseInput
}

// WindowsSendInput scrolls the foreground window by injecting synthetic
// mouse wheel events via the Win32 SendInput API.
//
// Before each batch of wheel events the cursor is moved to the centre of
// the foreground window. Windows routes MOUSEEVENTF_WHEEL to the window
// under the cursor (not keyboard focus), so this step is mandatory — without
// it the scroll goes to whatever window the user happened to leave the mouse
// over last, which is almost never the window being captured.
type WindowsSendInput struct{}

func NewWindowsSendInput() (*WindowsSendInput, error) {
	return &WindowsSendInput{}, nil
}

func (s *WindowsSendInput) Name() string { return "windows-sendinput" }

// Available always returns true — user32.dll is a guaranteed system DLL
// on any version of Windows this binary can run on.
func (s *WindowsSendInput) Available() bool { return true }

// ScrollDown injects synthetic downward wheel events equivalent to amountPx
// pixels of scroll distance.
func (s *WindowsSendInput) ScrollDown(amountPx int) error {
	if amountPx <= 0 {
		return errors.New("scroll amount must be positive")
	}

	// Move the cursor to the centre of the foreground window so Windows
	// routes the wheel events to the correct application.
	hwnd, _, _ := procGetFgWindow.Call()
	if hwnd == 0 {
		return fmt.Errorf("GetForegroundWindow: no foreground window")
	}

	var r winRECT
	ret, _, err := procGetWinRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return fmt.Errorf("GetWindowRect failed: %w", err)
	}

	cx := (int(r.left) + int(r.right)) / 2
	cy := (int(r.top) + int(r.bottom)) / 2
	ret, _, err = procSetCursorPos.Call(uintptr(cx), uintptr(cy))
	if ret == 0 {
		return fmt.Errorf("SetCursorPos failed: %w", err)
	}

	// Convert pixels to wheel notches and send one click at a time.
	// Batching all notches into a single SendInput call with a large
	// mouseData value works in theory, but some applications cap incoming
	// wheel deltas at one WHEEL_DELTA per event; firing individually is
	// safer and matches how real hardware behaves.
	notches := amountPx / pixelsPerWheelNotch
	if notches < 1 {
		notches = 1
	}

	// mouseData for MOUSEEVENTF_WHEEL is a signed value reinterpreted as
	// DWORD: negative = scroll toward the user (down). Go's constant folder
	// rejects uint32(-120) directly, so we go through a typed int32 variable
	// first to produce the correct 0xFFFFFF88 bit pattern.
	delta := int32(-wheelDelta)
	rec := inputRecord{
		inputType: inputMouse,
		mi: mouseInput{
			dwFlags:   mouseeventfWheel,
			mouseData: uint32(delta),
		},
	}
	sz := uintptr(unsafe.Sizeof(rec))

	for i := 0; i < notches; i++ {
		n, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&rec)), sz)
		if n == 0 {
			return fmt.Errorf("SendInput failed on notch %d: %w", i+1, err)
		}
	}

	return nil
}

// Close is a no-op — unlike the Linux uinput backend there is no kernel
// device handle to release.
func (s *WindowsSendInput) Close() error { return nil }

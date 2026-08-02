//go:build windows

package capture

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// windowsScreenshot captures via direct Win32 API calls (GDI + PrintWindow),
// using only Go's standard library (syscall/unsafe) — no external module
// dependencies. This was a deliberate choice over pulling in a screenshot
// library: it keeps cross-compilation simple (pure syscalls, no cgo) and
// avoids depending on a third-party Windows binding at all.
//
// Active window: GetForegroundWindow + GetWindowRect.
// Capture: PrintWindow with PW_RENDERFULLCONTENT, which — unlike a plain
// BitBlt off the window's device context — correctly captures modern,
// GPU-composited windows (browsers, Electron apps, anything using DWM
// composition) instead of returning a black or blank image for them.
type windowsScreenshot struct{}

func init() {
	Register(&windowsScreenshot{})
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procPrintWindow         = user32.NewProc("PrintWindow")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procGetDIBits           = gdi32.NewProc("GetDIBits")
	_                       = kernel32
)

type rect struct {
	left, top, right, bottom int32
}

type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type bitmapInfo struct {
	header bitmapInfoHeader
	// colors [1]uint32 — unused at 32bpp, omitted
}

const (
	pwRenderFullContent = 0x00000002
	biRGB               = 0
	dibRGBColors        = 0
)

func (w *windowsScreenshot) Name() string {
	return "windows"
}

func (w *windowsScreenshot) Available() bool {
	// If this binary is running at all under GOOS=windows, user32/gdi32
	// are always-present system DLLs — no meaningful failure mode to
	// check beyond that.
	return true
}

func (w *windowsScreenshot) CaptureActiveWindow() (image.Image, error) {
	// Must be called before any DPI-dependent coordinate work, or window
	// rects come back scaled and mismatched against the actual pixel
	// grid on displays with non-100% scaling.
	procSetProcessDPIAware.Call()

	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, fmt.Errorf("no foreground window found")
	}

	var r rect
	ret, _, err := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return nil, fmt.Errorf("GetWindowRect failed: %w", err)
	}
	width := int(r.right - r.left)
	height := int(r.bottom - r.top)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("window has invalid dimensions (%dx%d)", width, height)
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC(screen) failed")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdcMem)

	hBitmap, _, _ := procCreateCompatibleBmp.Call(hdcScreen, uintptr(width), uintptr(height))
	if hBitmap == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hBitmap)

	procSelectObject.Call(hdcMem, hBitmap)

	// PW_RENDERFULLCONTENT: correctly captures DWM-composited windows
	// (browsers, Electron apps) that a plain BitBlt would return blank.
	printed, _, _ := procPrintWindow.Call(hwnd, hdcMem, uintptr(pwRenderFullContent))
	if printed == 0 {
		return nil, fmt.Errorf("PrintWindow failed — window may not support rendering to a device context")
	}

	bi := bitmapInfo{
		header: bitmapInfoHeader{
			width:       int32(width),
			height:      -int32(height), // negative = top-down row order
			planes:      1,
			bitCount:    32,
			compression: biRGB,
		},
	}
	bi.header.size = uint32(unsafe.Sizeof(bi.header))

	buf := make([]byte, width*height*4)
	ret, _, err = procGetDIBits.Call(
		hdcMem, hBitmap, 0, uintptr(height),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bi)),
		uintptr(dibRGBColors),
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed: %w", err)
	}

	return bgraBufToRGBA(buf, width, height), nil
}

// bgraBufToRGBA converts a raw top-down BGRA pixel buffer (Windows DIB
// native format) into a standard image.RGBA, swapping red and blue.
func bgraBufToRGBA(buf []byte, width, height int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			if i+3 >= len(buf) {
				continue
			}
			blue := buf[i+0]
			green := buf[i+1]
			red := buf[i+2]
			alpha := buf[i+3]
			j := out.PixOffset(x, y)
			out.Pix[j+0] = red
			out.Pix[j+1] = green
			out.Pix[j+2] = blue
			out.Pix[j+3] = alpha
		}
	}
	return out
}
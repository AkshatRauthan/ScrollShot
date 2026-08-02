//go:build linux

package capture

import (
	"fmt"
	"image"
	"os"

	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgbutil"
	"github.com/jezek/xgbutil/ewmh"
	"github.com/jezek/xgbutil/xgraphics"
)

// x11Screenshot captures via a direct X11 protocol connection (pure Go —
// no cgo, no Xlib headers required at build time, unlike most X11
// screenshot tools). It finds the active window through the EWMH
// _NET_ACTIVE_WINDOW convention (the same mechanism window managers use)
// and captures it with a real GetImage request — the same underlying
// mechanism tools like maim/scrot use via Xlib, just reached through pure
// Go X protocol bindings instead.
//
// Deliberately gated to X11 sessions only (see Available): while an X
// server is often technically reachable under Wayland via XWayland, a
// GetImage request against a native Wayland client's window through that
// compatibility layer isn't reliable — capturing garbage or a blank
// surface for genuinely Wayland-native windows is worse than not
// offering this backend there at all. On GNOME Wayland the portal/
// gnome-screenshot backends are already registered and preferred.
type x11Screenshot struct{}

func init() {
	Register(&x11Screenshot{})
}

func (x *x11Screenshot) Name() string {
	return "x11"
}

func (x *x11Screenshot) Available() bool {
	if os.Getenv("XDG_SESSION_TYPE") != "x11" {
		return false
	}
	xu, err := xgbutil.NewConn()
	if err != nil {
		return false
	}
	xu.Conn().Close()
	return true
}

func (x *x11Screenshot) CaptureActiveWindow() (image.Image, error) {
	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connecting to X server: %w", err)
	}
	defer xu.Conn().Close()

	win, err := ewmh.ActiveWindowGet(xu)
	if err != nil {
		return nil, fmt.Errorf("getting active window (_NET_ACTIVE_WINDOW): %w", err)
	}
	if win == 0 {
		return nil, fmt.Errorf("no active window reported by the window manager")
	}

	ximg, err := xgraphics.NewDrawable(xu, xproto.Drawable(win))
	if err != nil {
		return nil, fmt.Errorf("capturing window %d: %w", win, err)
	}

	return bgraToRGBA(ximg), nil
}

// bgraToRGBA converts an xgraphics.Image (pixels stored in BGRA order,
// per the X server's native format) into a standard image.RGBA, swapping
// the red and blue channels.
func bgraToRGBA(src *xgraphics.Image) *image.RGBA {
	b := src.Rect
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := (y-b.Min.Y)*src.Stride + (x-b.Min.X)*4
			if i+3 >= len(src.Pix) {
				continue
			}
			blue := src.Pix[i+0]
			green := src.Pix[i+1]
			red := src.Pix[i+2]
			alpha := src.Pix[i+3]
			j := out.PixOffset(x, y)
			out.Pix[j+0] = red
			out.Pix[j+1] = green
			out.Pix[j+2] = blue
			out.Pix[j+3] = alpha
		}
	}
	return out
}

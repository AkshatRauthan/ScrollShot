//go:build linux

package capture

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
)

// gnomeScreenshot captures via the `gnome-screenshot` CLI, which talks to
// GNOME Shell's own org.gnome.Shell.Screenshot D-Bus interface. This works
// on GNOME Wayland (unlike grim/slurp, which need wlr-layer-shell — a
// protocol Mutter doesn't implement) because GNOME Shell has direct
// framebuffer access and hands back the image itself, no compositor
// protocol negotiation required.
//
// Limitation: depends on the gnome-screenshot binary being installed, and
// only works on GNOME specifically. The planned portal.go backend (direct
// org.freedesktop.portal.Screenshot D-Bus client) will remove both of
// those constraints and work across GNOME, KDE, and any other
// portal-compliant desktop.
type gnomeScreenshot struct{}

func init() {
	Register(&gnomeScreenshot{})
}

func (g *gnomeScreenshot) Name() string {
	return "gnome-screenshot"
}

func (g *gnomeScreenshot) Available() bool {
	_, err := exec.LookPath("gnome-screenshot")
	return err == nil
}

func (g *gnomeScreenshot) CaptureActiveWindow() (image.Image, error) {
	tmpFile, err := os.CreateTemp("", "scrollshot-capture-*.png")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// -w = active window only; -f = save to this path instead of ~/Pictures
	cmd := exec.Command("gnome-screenshot", "-w", "-f", tmpPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gnome-screenshot failed: %w\n%s", err, out)
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("opening captured file: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding captured PNG: %w", err)
	}
	return img, nil
}

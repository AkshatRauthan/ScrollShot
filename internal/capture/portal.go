//go:build linux

package capture

import (
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// portalScreenshot captures via the freedesktop desktop portal
// (org.freedesktop.portal.Screenshot), the standardized cross-desktop
// screenshot API that GNOME's own `gnome-screenshot` uses internally.
// Talking to it directly means:
//   - no dependency on the gnome-screenshot binary being installed
//   - works on any portal-compliant desktop, not just GNOME — KDE Plasma,
//     and others that implement xdg-desktop-portal, work identically
//
// Trade-off vs the gnome.go backend: the portal's Screenshot API captures
// the *whole screen*, not a specific window. There is no generic
// "active window only" concept in the portal spec — that's a GNOME-Shell-
// specific feature gnome-screenshot exposes via its own extra D-Bus
// interface, not something the standard portal offers. Some desktops'
// portal implementations show an interactive picker (including a window
// option) when "interactive" is set, but that requires the user to
// confirm a dialog on every single capture — unworkable for a
// hotkey-driven repeated-capture workflow. So this backend takes a full
// screen capture; cropping to a specific window/region is left to the
// planned internal/edit package.
type portalScreenshot struct{}

func init() {
	Register(&portalScreenshot{})
}

func (p *portalScreenshot) Name() string {
	return "xdg-desktop-portal"
}

func (p *portalScreenshot) Available() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	obj := conn.Object("org.freedesktop.portal.Desktop", dbus.ObjectPath("/org/freedesktop/portal/desktop"))
	// org.freedesktop.DBus.Peer.Ping confirms the portal service itself
	// is reachable, without triggering any screenshot/permission dialog.
	call := obj.Call("org.freedesktop.DBus.Peer.Ping", 0)
	return call.Err == nil
}

func (p *portalScreenshot) CaptureActiveWindow() (image.Image, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("connecting to session bus: %w", err)
	}

	// Subscribe to Request.Response signals before making the call, so we
	// can't miss the response in the (unlikely but possible) race where it
	// arrives before we'd otherwise start listening.
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return nil, fmt.Errorf("subscribing to portal signals: %w", err)
	}
	sigChan := make(chan *dbus.Signal, 10)
	conn.Signal(sigChan)
	defer conn.RemoveSignal(sigChan)

	obj := conn.Object("org.freedesktop.portal.Desktop", dbus.ObjectPath("/org/freedesktop/portal/desktop"))

	handleToken := fmt.Sprintf("scrollshot%d", rand.Intn(1_000_000))
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
		"interactive":  dbus.MakeVariant(false),
	}

	var requestPath dbus.ObjectPath
	call := obj.Call("org.freedesktop.portal.Screenshot.Screenshot", 0, "", options)
	if call.Err != nil {
		return nil, fmt.Errorf("calling portal Screenshot: %w", call.Err)
	}
	if err := call.Store(&requestPath); err != nil {
		return nil, fmt.Errorf("reading portal request handle: %w", err)
	}

	// Wait for the matching Response signal, with a timeout in case the
	// desktop shows a permission dialog the user never confirms.
	timeout := time.After(30 * time.Second)
	for {
		select {
		case sig := <-sigChan:
			if sig.Path != requestPath || sig.Name != "org.freedesktop.portal.Request.Response" {
				continue
			}
			return decodePortalResponse(sig)
		case <-timeout:
			return nil, fmt.Errorf("timed out waiting for portal response (was a permission dialog left unconfirmed?)")
		}
	}
}

func decodePortalResponse(sig *dbus.Signal) (image.Image, error) {
	if len(sig.Body) < 2 {
		return nil, fmt.Errorf("unexpected portal response format")
	}
	responseCode, ok := sig.Body[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("unexpected portal response code type")
	}
	if responseCode != 0 {
		return nil, fmt.Errorf("portal capture was cancelled or denied (code %d)", responseCode)
	}

	results, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("unexpected portal results format")
	}
	uriVariant, ok := results["uri"]
	if !ok {
		return nil, fmt.Errorf("portal response missing screenshot uri")
	}
	uri, ok := uriVariant.Value().(string)
	if !ok {
		return nil, fmt.Errorf("unexpected portal uri type")
	}

	path := strings.TrimPrefix(uri, "file://")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening portal screenshot file: %w", err)
	}
	defer f.Close()
	defer os.Remove(path) // portal writes to a temp location; clean up after reading

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding portal screenshot: %w", err)
	}
	return img, nil
}
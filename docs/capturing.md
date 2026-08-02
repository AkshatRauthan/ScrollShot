# How capturing works across platforms

Every backend implements the same three-method interface (see [`docs/architecture.md`](architecture.md) for the interface itself). This document explains what each one actually does under the hood, and why it's built the way it is.

## Backend selection

`capture.Detect()` returns the first registered backend whose `Available()` reports true, in file-registration order (roughly alphabetical: `gnome.go` before `portal.go` before `x11.go` before `windows.go`, within what a given build actually compiles). There's no `runtime.GOOS` switch anywhere — platform dispatch happens through Go build tags at compile time (a Windows build never even contains the GNOME/X11 code) plus each backend's own runtime `Available()` check.

You can force a specific backend for testing via:
```bash
SCROLLSHOT_BACKEND=x11 scrollshot capture
```

## `gnome-screenshot` (Linux, GNOME)

**File:** `internal/capture/gnome.go` · **Build tag:** `linux`

Shells out to the `gnome-screenshot` CLI with `-w` (active window only). This works on GNOME Wayland specifically because `gnome-screenshot` talks to GNOME Shell's own `org.gnome.Shell.Screenshot` D-Bus interface — GNOME Shell has direct framebuffer access and hands back the image itself, with no compositor protocol negotiation required from the caller.

**Why this exists at all, given the portal backend does something similar:** this was the original, first-built backend — the direct fix for the project's founding problem (`grim`/`slurp` failing on Mutter). It's GNOME-specific and requires the `gnome-screenshot` binary to be installed, both of which the portal backend later removed as constraints. It's kept because it's simple, well-tested, and still the first thing that works on a stock GNOME install.

**Limitation:** GNOME-only, and requires the binary present.

## D-Bus portal (Linux, any portal-compliant desktop)

**File:** `internal/capture/portal.go` · **Build tag:** `linux`

Talks directly to `org.freedesktop.portal.Screenshot` over D-Bus — the same standardized, cross-desktop API `gnome-screenshot` itself calls internally. Talking to it directly means no dependency on the `gnome-screenshot` binary, and native support for any desktop implementing the portal spec — KDE Plasma included, not just GNOME.

**The real tradeoff:** the standard portal API has no concept of "capture just the active window" — that's a GNOME-Shell-specific extra feature `gnome-screenshot` exposes, not something the generic portal spec offers. Some desktops' portal implementations show an interactive picker (including a window option) when `interactive: true` is set, but that requires a user-confirmed dialog on *every single capture*, which is unworkable for a hotkey-driven repeated-capture workflow. So this backend captures the **whole screen**, not a specific window — cropping to a specific window/region is left to the planned `internal/edit` package.

**Response handling:** subscribes to `org.freedesktop.portal.Request.Response` signals *before* making the call (to avoid a race where the response arrives before the listener is attached), with a 30-second timeout in case a permission dialog is shown and left unconfirmed.

## X11 (any Linux X11 desktop)

**File:** `internal/capture/x11.go` · **Build tag:** `linux`

Pure-Go X11 protocol client — `jezek/xgb`/`xgbutil` — deliberately chosen over the more common cgo+Xlib approach to keep cross-compilation simple and avoid requiring X11 development headers at build time.

**Active window:** found via the EWMH `_NET_ACTIVE_WINDOW` convention — the same mechanism window managers themselves use, so it works across window manager implementations without per-WM special-casing.

**Capture:** a real `xproto.GetImage` protocol request — the same underlying mechanism tools like `maim`/`scrot` use via Xlib, just reached through pure Go bindings instead of C bindings.

**Deliberately gated to `XDG_SESSION_TYPE=x11` only.** An X server is often technically reachable under Wayland via XWayland, but a `GetImage` request against a native Wayland client's window through that compatibility layer isn't reliable — it can return garbage or a blank surface for genuinely Wayland-native windows. Rather than offer an unreliable capture, this backend simply reports itself unavailable outside a real X11 session; on GNOME Wayland, the `gnome-screenshot`/portal backends are already registered and preferred anyway.

**Pixel format:** the X server returns BGRA; converted to standard `image.RGBA` by swapping the red and blue channels.

## Windows (amd64, arm64)

**File:** `internal/capture/windows.go` · **Build tag:** `windows`

Raw Win32 syscalls via Go's standard library (`syscall`/`unsafe`) — zero external dependencies, by choice. (An earlier attempt used the `kbinani/screenshot` module, but its Windows path pulls in a transitive dependency that wasn't fetchable in the development sandbox's network environment; writing directly against the Win32 API turned out to be the more robust choice anyway, with fewer moving parts to keep cross-compiling.)

**Active window:** `GetForegroundWindow` + `GetWindowRect`.

**Capture:** `PrintWindow` with the `PW_RENDERFULLCONTENT` flag — deliberately *not* a plain `BitBlt` off the window's device context, which returns a blank or black image for modern GPU-composited windows (browsers, Electron apps, anything relying on DWM composition). `PW_RENDERFULLCONTENT` handles that correctly.

**DPI awareness:** calls `SetProcessDPIAware()` before any coordinate work. Without this, window rectangles come back scaled in a way that mismatches the actual pixel grid on displays with non-100% scaling, producing cropped or offset captures.

**Pixel format:** Windows' native DIB format is BGRA, bottom-up by default; a negative height in the bitmap header requests top-down row order directly from `GetDIBits`, avoiding a manual row-flip. Converted to `image.RGBA` the same way as the X11 backend.

**Verification status:** cross-compiles cleanly for both `windows/amd64` and `windows/arm64` and passes `go vet`, but has not yet been runtime-tested on real Windows hardware — the development environment had no Windows machine available. Compiling clean is real signal but isn't the same as confirmed-working; treat this backend as needing a real-world test pass.

## macOS

Not yet built. The intended approach (native capture via CoreGraphics, active-window lookup via `NSWorkspace`/`CGWindowListCopyWindowInfo`) will require cgo, since there's no pure-Go equivalent — which also means it can only be properly build-verified on an actual Mac or a macOS CI runner, unlike the Windows backend.

## Wayland compositors without a portal installed

Bare Sway, Hyprland, and similar wlroots-based compositors *can* implement `org.freedesktop.portal.Screenshot` via `xdg-desktop-portal-wlr`, but it isn't always installed by default. If it's absent, `capture.Detect()` correctly returns "no backend available" rather than crashing or silently failing — but there's currently no fallback backend (e.g. a `grim`/`slurp`-based one) for this specific case. See [`docs/known-limitations.md`](known-limitations.md).
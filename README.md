# Scrollshot

**Native scrolling screenshots — built because nothing else worked on GNOME Wayland, and grown into a cross-platform tool from there.**

Scrollshot captures long, scrollable content — web pages, design canvases, code editors, chat logs, documentation — as a single continuous image. It started as a fix for a real, specific gap: every existing scrolling-screenshot tool on Linux assumes a Wayland compositor protocol (`wlr-layer-shell`) that GNOME's Mutter deliberately doesn't implement, so `grim`/`slurp`/every tool built on them simply fails there with no clean fallback.

---

## Why this matters

### For agentic coding
AI coding agents increasingly need to *see* what they're building — a rendered web app, a long settings page, a multi-screen user flow. A single viewport screenshot only shows a fraction of the actual UI state. Scrollshot gives an agent (or the human directing one) a complete, continuous view of a page or window in one image — useful for visual QA loops, regression comparison, and grounding an agent's understanding of what it actually shipped.

### For UI/UX design
Full-page mockups, design systems, and long user flows are painful to document with viewport-limited tools. Scrollshot captures an entire scrollable canvas in one clean image, ready to drop into a spec doc or design review without manually stitching screenshots.

### For everything else
QA and bug reports, documentation, code review of long files/diffs, research and archiving, support and compliance evidence — anywhere "the whole thing in one image" beats a stitched-together folder of overlapping crops.

---

## How it works

1. **Capture** — each keypress (or command) grabs the currently focused window through whatever mechanism actually works on your OS/desktop. See [`docs/capturing.md`](docs/capturing.md) for exactly how each platform is handled.
2. **Stack frames** — you scroll a bit between captures; each frame naturally overlaps the previous one.
3. **Stitch** — a calibrated matching engine finds exactly where consecutive frames overlap, detects and strips fixed UI (sticky navs, status bars) that shouldn't repeat, and glues everything into one continuous image with no visible seams. Full detail in [`docs/stitching.md`](docs/stitching.md).
4. **Save** — the finished image lands in your Pictures/Screenshots folder, ready to use.

Verified against real captures up to 23 frames at 2.8K resolution, and against real reported bugs on both a marketing web page and VS Code's dark editor — see [`CHANGELOG.md`](CHANGELOG.md) for the specifics.

---

## Quick Start

```bash
# Automatic — switch to the window, then run:
scrollshot auto
# Waits 5s, scrolls to the bottom, stitches, and saves automatically.

# Manual — capture frame by frame:
scrollshot capture   # switch to window first; repeats as many times as you scroll
scrollshot capture
scrollshot finish    # stitch and save
```

Output is saved to `~/Pictures/Screenshots/` (or the first writable fallback).

See **[`docs/installation.md`](docs/installation.md)** to get set up, and **[`docs/usage.md`](docs/usage.md)** for the full command reference, flags, troubleshooting, and workflows.

---

## Platform support

| Platform | Backend | Status |
|---|---|---|
| GNOME (Wayland or X11) | `gnome-screenshot` | ✅ Working |
| Any portal-compliant desktop (GNOME, KDE) | D-Bus `xdg-desktop-portal` | ✅ Working |
| Any X11 desktop | Pure-Go X11 protocol | ✅ Working |
| Windows (amd64, arm64) | Raw Win32 syscalls | ✅ Working |
| macOS | — | 🚧 Not yet built |
| Wayland compositors without `xdg-desktop-portal-*` installed (bare Sway/Hyprland) | — | ⚠️ Known gap |

**In plain terms, if you're running:**

| OS | Status | Comments |
|---|---|---|
| Ubuntu, Fedora Workstation, Debian (GNOME), Pop!_OS | ✅ Works out of the box | Uses the GNOME backend |
| Kubuntu, KDE neon, openSUSE (KDE Plasma) | ✅ Works out of the box | Uses the D-Bus portal backend |
| Arch/Manjaro with GNOME or KDE Plasma | ✅ Works out of the box | Same as above, whichever desktop applies |
| Xfce, i3, or any distro running an X11 session | ✅ Works out of the box | Uses the pure-Go X11 backend |
| Sway, Hyprland (or other wlroots compositors) | ⚠️ Conditional | Needs `xdg-desktop-portal-wlr` (or equivalent) installed — not there by default on every distro; see [`docs/known-limitations.md`](docs/known-limitations.md) |
| Windows 10/11 (amd64 or arm64) | ✅ Works out of the box | Uses raw Win32 syscalls |
| macOS (Intel or Apple Silicon) | 🚧 Not supported yet | No backend built yet |

Not sure which category your setup falls into? Just run `scrollshot capture` — it auto-detects and tells you plainly if nothing works for your environment.

The right backend is selected automatically at runtime — see [`docs/capturing.md`](docs/capturing.md) for how.

---

## Architecture

```
scrollshot/
├── cmd/scrollshot/        CLI entrypoint
└── internal/
    ├── capture/             pluggable screenshot backends, one per OS/protocol
    ├── session/             frame staging between capture and finish
    ├── stitch/              the matching + stitching engine
    ├── autoscroll/          driven scrolling + end-of-page detection (Windows & Linux)
    ├── paths/               OS-aware session and output directory resolution
    ├── debug/               centralized structured logging
    ├── notify/              desktop notifications after finish
    ├── edit/                [planned] crop / reorder frames
    └── export/              [planned] lossless & lossy size control
```

See [`docs/architecture.md`](docs/architecture.md) for the reasoning behind this shape, and [`CONTRIBUTING.md`](CONTRIBUTING.md) if you want to add a backend or feature yourself.

---

## Roadmap

- **macOS backend** — deferred, next up when picked back up
- **Wayland fallback backend** (`grim`/`slurp`) for wlroots compositors without a portal installed
- **Frame cropping & reordering** — manual controls on top of the automatic fixed-UI detection already in place
- **Lossless & lossy export control** — tighter compression or scaled-down output when file size matters more than pixel-perfect fidelity

✅ **Autoscroll** (`scrollshot auto`) shipped in v0.3.0 — Windows and Linux, fully automatic.

## Known limitations

A few edge cases don't have clean answers yet — see [`docs/known-limitations.md`](docs/known-limitations.md) for the honest list rather than pretending they don't exist.

## Community

This started as a fix for a problem GNOME Wayland users have been hitting for years with no clean answer, and grew from there.

- **Try it, break it, file an issue** — especially on desktop environments and distros beyond what's already been tested
- **Contribute a backend or feature** — see [`CONTRIBUTING.md`](CONTRIBUTING.md); the `Capturer` interface is small and self-contained, and a new backend never requires touching existing code
- **Share how you're using it** — shapes what gets prioritized next

## License

MIT — see [`LICENSE`](LICENSE).
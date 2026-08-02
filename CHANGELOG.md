# Changelog

All notable changes to this project are documented here. The format loosely follows [Keep a Changelog](https://keepachangelog.com/).

## v0.2.0 — Cross-platform support, improved stitching, and documentation

### Added
- Native Windows capture backend using raw Win32 APIs (amd64, arm64).
- Pure-Go X11 capture backend with no cgo or Xlib dependencies.
- `xdg-desktop-portal` capture backend for portal-compliant Linux desktops.
- Automatic detection and removal of fixed UI (sticky headers, status bars) before stitching.
- `SCROLLSHOT_BACKEND` environment variable to manually select a capture backend.
- Validation for mismatched frame dimensions during stitching.
- Project documentation covering architecture, capture backends, stitching, and known limitations.

### Improved
- Reworked the stitching algorithm for greater accuracy on real-world captures.
- Better handling of repetitive content such as code editors and large uniform backgrounds.
- Improved resilience to anti-aliasing and font-rendering differences between captures.
- More reliable detection of fixed UI across capture sessions.
- Reduced over-cropping by refining static-edge detection thresholds.

### Fixed
- Prevent corrupted output when frame dimensions change during a capture session.
- Eliminated false overlap matches caused by background-dominated images.
- Reduced incorrect matches on repetitive content.
- Fixed valid matches being rejected because of rendering noise.
- Improved handling of stale capture sessions by automatically clearing unfinished sessions.

### Changed
- Reorganized the project into modular packages (`capture`, `session`, `stitch`, `edit`, `export`, and `autoscroll`).
- Separated stitching and static-edge detection tolerances for easier tuning.
- Adjusted static-edge detection limits to better match real-world interfaces.

---

## v0.1.0 — Initial release

### Added
- Initial GNOME capture backend using `gnome-screenshot`.
- Session management with `capture` and `finish` commands.
- Initial overlap-based image stitching engine.
- Automatic cleanup of stale capture sessions.

### Notes
- Created to address the lack of reliable scrolling screenshots on GNOME Wayland, where existing `grim`/`slurp`-based tools cannot operate due to unsupported compositor protocols.
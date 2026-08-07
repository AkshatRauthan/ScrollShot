# Changelog

## v0.4.0 — Package distribution: apt, Scoop, and winget

### Added
- Self-hosted apt repository (`apt/conf/distributions`) built with `reprepro`, GPG-signed, and published to GitHub Pages on every tagged release.
- `.deb` and `.rpm` packaging via `nfpm` (`nfpm.yaml`), built for `linux/amd64` alongside the existing binary release.
- Scoop bucket manifest (`bucket/scrollshot.json`) for Windows, hosted directly in this repo with `autoupdate` wired to `SHA256SUMS.txt`.
- winget manifest set (`winget/`) prepared for submission to the official `winget-pkgs` repository.

### Changed
- Release workflow (`release.yml`) now rebuilds and republishes the apt repository and commits the updated Scoop manifest automatically after each tagged release.
- Installation instructions split out of `docs/usage.md` into a dedicated `docs/installation.md`, covering all install methods (binary, package managers, build from source, PATH setup, and Linux `uinput` permissions).

### Notes
- winget listing is pending review/merge in `microsoft/winget-pkgs` — not installable via `winget install` until merged.
- apt and Scoop are both self-hosted; no third-party review required for either.

---

## v0.3.0 — Autoscroll, debug logging, and Windows stability

### Added
- `scrollshot auto` command: full scroll-capture-stitch in a single step on Windows and Linux.
- Native Windows autoscroll backend via `SendInput` Win32 API.
- Native Linux autoscroll backend via `uinput` kernel module (X11 and Wayland).
- Centralized debug logging (`internal/debug`) with `--debug` flag and `SCROLLSHOT_DEBUG=1` env var.
- Desktop notifications on capture and finish completion.
- Output directory fallback chain (`Pictures/Screenshots` → `Pictures` → `scrollshot_output`) with live write-probe to handle OneDrive and Controlled Folder Access on Windows.

### Improved
- Windows GDI capture: bitmap deselected from DC before `GetDIBits` to prevent silent zero-scanline failures.
- Windows home directory resolution falls back through `%USERPROFILE%`, `%LOCALAPPDATA%`, and `%APPDATA%`.
- Debug logging instrumented across `session`, `stitch`, `capture/windows`, `paths`, and `autoscroll`.
- Errors wrapped consistently with `fmt.Errorf("context: %w", err)` throughout the codebase.

### Fixed
- `GetDIBits` failing silently when bitmap was still selected into the device context.
- `os.Create` failing with "The system cannot find the file specified" for OneDrive-managed and junction-target directories.
- Output saved to `C:\Users\Pictures\...` (missing username) due to `HOMEDRIVE`+`HOMEPATH` resolving without the user profile subdirectory.

### Refactored
- Autoscroll logic split into controller, config, and result packages.
- Session directory resolution moved to `internal/paths` for OS-aware cache location.

---

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

### Refactored
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
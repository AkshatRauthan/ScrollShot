# Known limitations

This is a deliberately honest list. Everything here was found through actual testing (usually a synthetic stress test built specifically to probe for it), not theorized — and none of it is silent. Where a limitation affects real usage, the tool tells you (a warning in the output log), rather than failing quietly.

## Stitching engine

### Fully-identical consecutive frames over-crop

If two consecutive captures are byte-identical (typically: pressing capture twice without scrolling at all in between), static-UI detection can't distinguish "this edge region is fixed UI" from "the whole frame happens to be identical this time" — with only two fully-matching frames, every region technically satisfies the static-detection test up to the configured cap (12% of frame height from each edge).

**Effect:** the final output can be noticeably shorter than the true content, having cropped real (but coincidentally-static-looking) content from both edges.

**Why it isn't fully fixed:** this is a genuine information-theoretic ambiguity, not a tuning problem — there's no way to distinguish the two cases from pixels alone when the frames are truly identical. A more complete fix would need session-level context (e.g. detecting that a "match" spanning nearly the entire frame height is itself suspicious and should suppress static-cropping), which hasn't been built yet.

**Practical impact:** low. This specifically requires zero scroll between two captures, which is an unusual capture pattern in normal use.

### Small residual imprecision on extreme low-signal repetitive content

On synthetic test content combining very sparse distinguishing detail with strong repetitive structure, the matcher has been observed to find an overlap a small number of pixels off from the true value (tens of pixels, not hundreds) rather than the exact figure.

**Practical impact:** low. Real-world testing (a marketing web page, a VS Code session) has consistently produced exact matches; this was only observed on deliberately adversarial synthetic data designed to stress-test the sampling approach.

### Static UI is cropped entirely, not shown once

When a sticky nav or fixed status bar is detected, it's removed from **every** frame, including the first and last — it won't appear even once in the final output, rather than being shown once at the top/bottom for context.

**Why:** this is a deliberate default, not an oversight. It's the simplest, most predictable behavior, and matches how most scrolling-screenshot tools handle sticky elements. If you want a header shown once for context, that's a reasonable feature request but isn't currently configurable.

### Portal backend captures the whole screen, not a window

Covered in detail in [`docs/capturing.md`](capturing.md) — the standard `xdg-desktop-portal` Screenshot API has no window-specific capture mode. This isn't a bug so much as a real constraint of the underlying API; cropping to a window after the fact is planned work for `internal/edit`, not yet built.

## Platform coverage

### macOS is not implemented

No capture backend exists for macOS yet. Deliberately deferred — the intended approach requires cgo (no pure-Go path exists for CoreGraphics), which also means it can't be build-verified without access to an actual Mac or macOS CI runner, unlike every backend built so far.

### Wayland compositors without a portal installed

Bare Sway, Hyprland, and other wlroots-based compositors can support scrolling capture via `xdg-desktop-portal-wlr`, but that package isn't installed by default on every distro. If it's missing, `capture.Detect()` correctly reports no backend is available rather than failing unpredictably — but there's no fallback (e.g. a direct `grim`/`slurp`-based backend) for this case yet. In practice, users on these compositors typically already have `grim`/`slurp` set up directly and don't need this tool for basic screenshots — the gap mainly affects users who've moved to Scrollshot specifically for the scrolling-capture feature without also having the portal installed.

### Windows backend is compile-verified only

The Windows capture backend cross-compiles cleanly for `amd64` and `arm64` and passes static analysis (`go vet`), but has not been run on real Windows hardware during development — no Windows machine was available in the environment it was built in. The `PrintWindow`/`GetDIBits` calls and the BGRA→RGBA pixel conversion are implemented per Win32 documentation and are believed correct, but "compiles clean" and "confirmed working at runtime" are different claims. If you test this on real Windows hardware and hit an issue, that's genuinely useful signal, not a redundant bug report.

### X11 backend is XWayland-incompatible by design, not by accident

The X11 backend deliberately refuses to activate under a Wayland session, even though an X server is often technically reachable via XWayland. See [`docs/capturing.md`](capturing.md) for why — this is a considered tradeoff (unreliable capture is worse than no capture), not a missing feature.

## What's simply not built yet

These aren't limitations in the sense of "doesn't work correctly" — they're planned features that don't exist yet: autoscroll/auto-capture, manual frame cropping and reordering, lossless/lossy export size control. See the Roadmap section of [`README.md`](../README.md) for current status.
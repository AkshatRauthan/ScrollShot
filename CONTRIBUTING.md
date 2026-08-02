# Contributing to Scrollshot

Thanks for considering it. This doc covers the two things people are most likely to want to do: add a capture backend for a new platform, or work on the stitching engine.

## Getting set up

```bash
git clone <repo>
cd scrollshot
go build -o scrollshot ./cmd/scrollshot
```

No external tools beyond the Go toolchain are required to build the Linux/Windows backends. The project deliberately favors zero or minimal dependencies — see "Dependency philosophy" below before reaching for a new module.

## Adding a capture backend

This is the easiest, most self-contained way to contribute. The whole system is built around one small interface in `internal/capture/capture.go`:

```go
type Capturer interface {
    Name() string
    Available() bool
    CaptureActiveWindow() (image.Image, error)
}
```

To add a backend:

1. Create a new file in `internal/capture/`, e.g. `internal/capture/yourplatform.go`.
2. Add a build tag restricting it to the right platform: `//go:build darwin` (or whatever applies).
3. Implement the three methods on a small struct.
4. Register it in an `init()`:
   ```go
   func init() {
       Register(&yourBackend{})
   }
   ```

That's it. You do not need to touch `main.go`, the registry, or any other backend — `capture.Detect()` automatically picks up anything registered, in file-registration order, choosing the first one whose `Available()` returns true. This is the whole point of the interface: every existing backend (`gnome.go`, `portal.go`, `x11.go`, `windows.go`) was added this way, and none of them know the others exist.

### `Available()` should be conservative

A backend claiming to be available when it isn't wastes the user's time with a confusing failure instead of a clean "not available here." Look at `x11.go` for the pattern: it checks `XDG_SESSION_TYPE` *and* actually attempts a real connection before returning true — a false positive here is worse than a false negative.

### Prefer zero or minimal dependencies

The Windows backend (`windows.go`) is written against raw Win32 syscalls using only the Go standard library — deliberately, not because a screenshot library wasn't available, but because it keeps cross-compilation simple and doesn't depend on a third-party binding staying maintained. The X11 backend uses `jezek/xgb`/`xgbutil` specifically because they're pure Go (no cgo, no Xlib headers needed at build time) — that was a conscious tradeoff over the more common `kbinani/screenshot`, which requires cgo on some platforms.

If your platform genuinely needs cgo (macOS via CoreGraphics likely will), that's fine — just isolate it to your one file behind its build tag, and say so clearly in a comment.

### Test what you can, say what you couldn't

Every backend added so far was verified with whatever tooling was actually available at the time — `go vet`, cross-compilation for the target `GOOS`/`GOARCH`, and manual testing on a real machine where one was accessible. If you can't test on real hardware, say so explicitly in your PR description rather than implying it's been verified. See the X11 and Windows backends' commit history for the honest pattern: "compiles clean and passes vet, but I don't have a real X11/Windows machine to confirm runtime behavior."

## Working on the stitching engine

`internal/stitch/stitch.go` is the most heavily-iterated part of this codebase, and it got that way by chasing real, reported bugs — not by guessing at good numbers. Before changing a tuning constant (`pointTolerance`, `minMatchScore`, `minMargin`, `highConfidenceOverride`, etc.), read [`docs/stitching.md`](docs/stitching.md) — it explains why each one is set where it is, including the specific false positives/negatives that shaped each value.

If you're tuning a constant:

1. **Build a synthetic test that reproduces the failure you're trying to fix.** Every fix in this project's history started with a small generator script producing test frames with a known, exact expected outcome — not eyeballing real screenshots.
2. **Re-run every previous test case, not just your new one.** This codebase's history includes multiple instances of a fix for one case quietly breaking another (background-domination fixes affecting static-edge detection, tolerance changes affecting both matching and static detection because they shared a constant). Assume your change has side effects until you've checked.
3. **When you find the right number, say how you found it.** "Set to 90 after sweeping 60/90/120/150/180 against a calibrated jitter test matching real observed scores" is useful to the next person. "Set to 90, seemed to work" is not.

## Style

- Run `gofmt -w` before committing.
- Doc comments on exported functions/types should explain *why*, not just restate the signature — see any existing file in `internal/stitch` or `internal/capture` for the expected level of detail.
- Keep platform-specific code behind build tags and inside its own file. Nothing outside `internal/capture` should ever need a `runtime.GOOS` check.

## Reporting bugs

If a scrolling capture comes out wrong, the most useful report includes:
- The full terminal log from `scrollshot finish` (the per-frame overlap/score output)
- The raw frame PNGs from the session directory (`~/.cache/scrollshot_session/` — captured *before* running `finish`, since it clears them on success)
- What you expected vs. what you got

This is exactly the level of detail that found and fixed every real bug in this project so far.
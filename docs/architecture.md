# Architecture

This document explains *why* the codebase is shaped the way it is, not just what's in each file — the folder listing alone doesn't tell you much.

## The shape

```
scrollshot/
├── cmd/scrollshot/main.go       CLI entrypoint — thin, does almost nothing itself
└── internal/
    ├── capture/                  "how do I grab a screenshot" — OS/protocol-specific
    ├── session/                  "where do captured frames live between commands"
    ├── stitch/                   "how do I glue frames together" — pure image logic
    ├── autoscroll/               "drive the scrolling itself" — controller + per-OS backends
    ├── paths/                    "where do session and output files go" — OS-aware paths
    ├── debug/                    "structured verbose logging" — single Enable() call
    ├── notify/                   "desktop notification after finish" — OS-specific
    ├── edit/                     "adjust frames before/after stitching" — planned
    └── export/                   "control output file size" — planned
```

Each package answers exactly one question. That's the organizing principle, and it's worth understanding because it's what makes the rest of the design decisions below make sense.

## `cmd/scrollshot/main.go` — deliberately thin

`main.go` only does argument parsing and wiring: it calls into `capture`, `session`, and `stitch`, and prints their results. It contains no business logic of its own. This isn't an aesthetic preference — it means every package underneath is independently testable and reusable without a CLI attached, and it means `main.go` almost never needs to change when a package's internals change. Compare the git history: `stitch.go` has been rewritten several times chasing real bugs; `main.go`'s call sites have barely moved.

## `internal/capture` — the interface-first package

This is the package most worth understanding if you're extending the project, because it demonstrates the pattern the whole codebase leans on.

```go
type Capturer interface {
    Name() string
    Available() bool
    CaptureActiveWindow() (image.Image, error)
}
```

Every backend (`gnome.go`, `portal.go`, `x11.go`, `windows.go`) implements this interface and registers itself in its own `init()`:

```go
func init() {
    Register(&x11Screenshot{})
}
```

Nothing else — not `main.go`, not the other backends, not the registry itself — needs to know a new backend exists. `capture.Detect()` walks whatever's registered and returns the first one whose `Available()` reports true. There is no `switch runtime.GOOS` anywhere in this codebase; platform dispatch happens entirely through Go build tags (`//go:build linux`, `//go:build windows`) plus this runtime availability check, which is why adding macOS support later will mean adding one new file, not touching four existing ones.

This also means a backend's `Available()` check is doing real work, not a formality — see `x11.go`'s two-layer check (session-type environment variable, then an actual connection attempt) for the expected level of care. A backend that claims availability incorrectly produces a confusing runtime failure instead of a clean "not available here, trying the next one."

## `internal/session` — deliberately separated from `main.go`

Session management (where frames get staged, stale-session auto-clearing) used to live directly in `main.go`. It was extracted into its own package specifically so the storage logic could be tested and reasoned about independently of CLI concerns — and because "where do frames live" is a genuinely different question from "what does the `capture` command do," even though early on they were the same function.

## `internal/stitch` — pure, and that's load-bearing

The stitching engine takes `image.RGBA` in and produces `image.RGBA` out. It has no knowledge of files, sessions, or capture backends. This isn't just clean separation for its own sake — it's what made the extensive testing this package has been through possible at all. Every fix documented in `CHANGELOG.md` started with a small Go program generating synthetic test frames with a known, exact expected outcome, feeding them directly into `stitch.Stitch()`, and checking the output — no session directories, no real screenshots, no capture backend needed. See [`docs/stitching.md`](stitching.md) for the algorithm itself.

## `internal/paths` and `internal/debug` — cross-cutting concerns

`internal/paths` answers "where does this file live?" for every platform. All session and output directory logic centralises here, with a fallback chain that probes writability by actually creating a file — the only reliable way to detect OneDrive-redirected or Controlled Folder Access directories on Windows without a TOCTOU race.

`internal/debug` is the single logging choke-point. Components call `debug.Logf("component", "...")`, and the entire output is gated behind one `Enable()` call, toggled by `--debug` or `SCROLLSHOT_DEBUG=1`. Nothing in the packages themselves decides whether logs are visible — that's always the CLI's decision.

`internal/notify` fires a desktop notification after `finish` or `auto` completes. It is build-tag gated (`notify_linux.go`, `notify_windows.go`, `notify_other.go`) so platform-specific mechanisms (libnotify on Linux, Windows toast API via PowerShell) are never compiled into the wrong binary.

## `internal/autoscroll` — mirrors `capture`'s per-backend pattern

`autoscroll` was designed from the start to mirror `capture`'s registry pattern because driving scroll input is just as OS/compositor-restricted as taking a screenshot. The package is split into:

- `controller.go` — the platform-agnostic scroll loop: frame capture, overlap detection, stagnation tracking, stop conditions
- `config.go` — tuneable parameters (`MaxFrames`, `Delay`, `ScrollFraction`, `MinimumAdvancePx`, `StagnationLimit`)
- `result.go` — the per-frame result type returned to the caller
- `backend/` — OS-gated backends: `linux_uinput.go` (kernel `uinput` module, works on X11 and Wayland), `windows.go` (`SendInput` Win32 API), `detect_*.go` (build-tag-gated factory functions)

The controller has no `switch runtime.GOOS` — it calls `backend.Detect()` and works with whatever `Scroller` comes back. Adding a macOS backend means adding one new file in `backend/`, not touching the controller.

## Why build tags instead of runtime OS checks everywhere

Every OS-specific file starts with a build constraint:

```go
//go:build windows
```

This means a Linux build of the binary never even compiles the Windows syscall code, and vice versa — there's no `unsafe.Pointer` Win32 struct layout sitting in a binary that will never run on Windows, and no risk of a stray `runtime.GOOS == "windows"` check being wrong or forgotten somewhere. The tradeoff is that you can't unit-test a Windows-only file on a Linux CI runner directly — which is why the Windows backend was verified via cross-compilation (`GOOS=windows go build`) rather than execution, and why a real Windows/X11 test pass by someone with the actual hardware remains valuable even after a clean cross-compile.
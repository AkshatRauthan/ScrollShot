// scrollshot — scrolling screenshot tool.
//
// This file is intentionally thin: argument parsing and wiring only.
// The real logic lives in internal/:
//   - capture:    pluggable screenshot backends, one per OS/compositor
//   - session:    where captured frames are staged between commands
//   - stitch:     pure image logic that glues frames together
//   - autoscroll: automatic scrolling + end-of-page detection (Linux & Windows)
//   - edit:       [planned] crop/reorder frames before stitching
//   - export:     [planned] lossless/lossy output size control
//
// Usage:
//
//	scrollshot capture [-wait N]   capture current window (waits N sec first, default 5)
//	scrollshot finish              stitch all captures, save final image, clear session
//	scrollshot auto    [-wait N]   automatic scroll-capture-stitch (waits N sec first, default 5)
//	scrollshot version             print version
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"scrollshot/internal/autoscroll"
	"scrollshot/internal/capture"
	"scrollshot/internal/notify"
	"scrollshot/internal/paths"
	"scrollshot/internal/session"
	"scrollshot/internal/stitch"
)

// version is set at build time via -ldflags "-X main.version=..." (see
// .github/workflows/release.yml). Defaults to "dev" for local builds.
var version = "dev"

func timestamp() string {
	return time.Now().Format("20060102_150405")
}

func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// defaultWait is the number of seconds scrollshot waits before executing
// a capture or auto command, giving the user time to switch focus to the
// target window.
const defaultWait = 5

// parseWait parses a -wait flag from args and returns the delay duration.
// Unknown flags cause the program to exit with usage information.
func parseWait(subcommand string, args []string) time.Duration {
	fs := flag.NewFlagSet(subcommand, flag.ExitOnError)
	wait := fs.Int("wait", defaultWait, "seconds to wait before executing")
	fs.Parse(args) //nolint:errcheck // ExitOnError means Parse never returns a non-nil error
	return time.Duration(*wait) * time.Second
}

func cmdCapture(wait time.Duration) {
	var backend capture.Capturer
	if forced := os.Getenv("SCROLLSHOT_BACKEND"); forced != "" {
		backend = capture.Get(forced)
		if backend == nil {
			die("unknown backend %q (available: %v)", forced, capture.List())
		}
		if !backend.Available() {
			die("backend %q is not available in this environment", forced)
		}
	} else {
		backend = capture.Detect()
	}
	if backend == nil {
		die("no capture backend available for this environment (tried: %v)", capture.List())
	}

	sess, err := session.New()
	if err != nil {
		die("could not initialize session: %v", err)
	}

	clearedStale, err := sess.EnsureFresh()
	if err != nil {
		die("could not prepare session: %v", err)
	}
	if clearedStale {
		fmt.Println("Previous session went stale — started a new one")
	}

	img, err := backend.CaptureActiveWindow()
	if err != nil {
		die("capture failed (%s): %v", backend.Name(), err)
	}

	idx, path, err := sess.SaveFrame(img)
	if err != nil {
		die("could not save frame: %v", err)
	}

	fmt.Printf("Captured frame %d via %s -> %s\n", idx, backend.Name(), path)
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func cmdFinish() {
	sess, err := session.New()
	if err != nil {
		die("could not initialize session: %v", err)
	}

	framePaths, err := sess.Frames()
	if err != nil {
		die("could not list session frames: %v", err)
	}
	if len(framePaths) == 0 {
		die("No frames captured. Run 'scrollshot capture' first.")
	}

	frames := make([]*image.RGBA, 0, len(framePaths))
	for _, p := range framePaths {
		img, err := loadImage(p)
		if err != nil {
			die("could not read %s: %v", p, err)
		}
		frames = append(frames, stitch.ToRGBA(img))
	}

	result, log, static, err := stitch.Stitch(frames)
	if err != nil {
		die("stitching failed: %v\n\nFrames were left in place at %s for inspection — fix the mismatch and run 'scrollshot finish' again, or delete the session and recapture.", err, sess.Dir())
	}
	if result == nil {
		die("stitching produced no output")
	}

	if static.TopRows > 0 || static.BottomRows > 0 {
		fmt.Printf("Detected fixed UI unchanged across all frames — cropped %dpx from top, %dpx from bottom\n", static.TopRows, static.BottomRows)
	}

	for _, r := range log {
		if r.Index == 0 {
			continue
		}
		warn := ""
		if r.LowConfident {
			warn = fmt.Sprintf("  ⚠ no confident match found (best score %.0f%%) — check this join for a duplicate or a gap", r.MatchScore*100)
		}
		fmt.Printf("Frame %d: matched %dpx overlap, added %dpx%s\n", r.Index, r.OverlapPx, r.AddedPx, warn)
	}

	outPath, err := saveOutput(result)
	if err != nil {
		die("could not save output: %v", err)
	}

	if err := sess.Clear(); err != nil {
		die("stitched successfully but could not clear session: %v", err)
	}

	fmt.Println("Saved:", outPath)
	sendNotification(outPath)
}

func saveOutput(img image.Image) (string, error) {
	dir, err := paths.OutputDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "scrolling_"+timestamp()+".png")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return path, nil
}

func cmdAuto(wait time.Duration) {
	if wait > 0 {
		fmt.Printf("Waiting %v before auto scroll...\n", wait)
		time.Sleep(wait)
	}

	// Honour SCROLLSHOT_BACKEND for consistency with cmdCapture.
	var backend capture.Capturer
	if forced := os.Getenv("SCROLLSHOT_BACKEND"); forced != "" {
		backend = capture.Get(forced)
		if backend == nil {
			die("unknown backend %q (available: %v)", forced, capture.List())
		}
		if !backend.Available() {
			die("backend %q is not available in this environment", forced)
		}
	} else {
		backend = capture.Detect()
	}
	if backend == nil {
		die("no capture backend available for this environment (tried: %v)", capture.List())
	}

	scroller, err := autoscroll.DetectScroller()
	if err != nil {
		die("%v", err)
	}
	defer scroller.Close()

	sess, err := session.New()
	if err != nil {
		die("could not initialize session: %v", err)
	}

	clearedStale, err := sess.EnsureFresh()
	if err != nil {
		die("could not prepare session: %v", err)
	}
	if clearedStale {
		fmt.Println("Previous session went stale — started a new one")
	}

	controller, err := autoscroll.New(
		backend,
		scroller,
		sess,
		autoscroll.DefaultConfig(),
	)
	if err != nil {
		die("could not initialize autoscroll: %v", err)
	}

	result, err := controller.Run()
	if err != nil {
		die("autoscroll failed: %v", err)
	}

	fmt.Printf(
		"Stoppage triggered due to (%s)\nCaptured %d frames\n",
		result.StopReason,
		result.FramesCaptured,
	)

	// Automatically stitch and export the captured session.
	cmdFinish()
}

func sendNotification(path string) {
	notify.Send("Scrolling screenshot saved", path)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: scrollshot [capture|finish|auto|version]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "capture":
		wait := parseWait("capture", os.Args[2:])
		cmdCapture(wait)
	case "finish":
		cmdFinish()
	case "auto":
		wait := parseWait("auto", os.Args[2:])
		cmdAuto(wait)
	case "version", "--version", "-v":
		fmt.Println("scrollshot", version)
	default:
		fmt.Println("Usage: scrollshot [capture|finish|auto|version]")
		os.Exit(1)
	}
}

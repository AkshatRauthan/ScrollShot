// scrollshot — scrolling screenshot tool.
//
// This file is intentionally thin: argument parsing and wiring only.
// The real logic lives in internal/:
//   - capture:    pluggable screenshot backends, one per OS/compositor
//   - session:    where captured frames are staged between commands
//   - stitch:     pure image logic that glues frames together
//   - edit:       [planned] crop/reorder frames before stitching
//   - export:     [planned] lossless/lossy output size control
//   - autoscroll: [planned] drive scrolling + detect end-of-page
//
// Usage:
//
//	scrollshot capture   capture current window state, add to session
//	scrollshot finish    stitch all captures, save final image, clear session
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"time"

	"scrollshot/internal/capture"
	"scrollshot/internal/session"
	"scrollshot/internal/stitch"
)

func timestamp() string {
	return time.Now().Format("20060102_150405")
}

func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func cmdCapture() {
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
	notify(outPath)
}

func outputDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := home + "/Pictures/Screenshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func saveOutput(img image.Image) (string, error) {
	dir, err := outputDir()
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/scrolling_%s.png", dir, timestamp())
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

func notify(path string) {
	defer func() { recover() }()
	exec.Command("notify-send", "Scrolling screenshot saved", path).Run()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: scrollshot [capture|finish]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "capture":
		cmdCapture()
	case "finish":
		cmdFinish()
	default:
		fmt.Println("Usage: scrollshot [capture|finish]")
		os.Exit(1)
	}
}

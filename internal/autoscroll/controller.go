package autoscroll

import (
	"errors"
	"fmt"
	"image"
	"time"

	"scrollshot/internal/autoscroll/backend"
	"scrollshot/internal/capture"
	"scrollshot/internal/debug"
	"scrollshot/internal/session"
	"scrollshot/internal/stitch"
)

// Controller coordinates automatic scrolling and screenshot capture.
type Controller struct {
	Capturer capture.Capturer
	Scroller backend.Scroller
	Session  *session.Session
	Config   Config

	previous       *image.RGBA
	framesCaptured int

	// stagnation counts consecutive frames where overlap advance is too small.
	stagnation int
	// noMatchStagnation counts consecutive frames where FindOverlap found no
	// confident match — triggered by identical frames at page bottom or by
	// highly animated content where the margin check always fails.
	noMatchStagnation int

	// viewportH is the effective scrollable height after removing static UI.
	// Zero until calibrated after the second frame.
	viewportH int

	// staticEdges are cached after calibration and used to crop frames
	// before overlap analysis — matching what Stitch does internally.
	staticEdges stitch.StaticEdges
}

// New validates dependencies and constructs a controller.
//
// Callers are responsible for calling sess.EnsureFresh() before Run —
// New does not call it to avoid a redundant double-call.
func New(
	c_arg capture.Capturer,
	s backend.Scroller,
	sess *session.Session,
	cfg Config,
) (*Controller, error) {

	if c_arg == nil {
		return nil, errors.New("nil capturer")
	}
	if s == nil {
		return nil, errors.New("nil scroller")
	}
	if sess == nil {
		return nil, errors.New("nil session")
	}

	if cfg.ScrollFraction <= 0 || cfg.ScrollFraction > 1 {
		cfg = DefaultConfig()
	}
	if cfg.Delay <= 0 {
		cfg.Delay = DefaultConfig().Delay
	}
	if cfg.MaxFrames <= 0 {
		cfg.MaxFrames = DefaultConfig().MaxFrames
	}
	if cfg.MinimumAdvancePx <= 0 {
		cfg.MinimumAdvancePx = DefaultConfig().MinimumAdvancePx
	}
	if cfg.StagnationLimit <= 0 {
		cfg.StagnationLimit = DefaultConfig().StagnationLimit
	}

	c := &Controller{
		Capturer: c_arg,
		Scroller: s,
		Session:  sess,
		Config:   cfg,
	}
	debug.Logf("auto", "initialized autoscroll controller (max_frames=%d, delay=%v, scroll_fraction=%.2f, min_advance_px=%d, stagnation_limit=%d)",
		cfg.MaxFrames, cfg.Delay, cfg.ScrollFraction, cfg.MinimumAdvancePx, cfg.StagnationLimit)
	return c, nil
}

// captureFrame captures the focused window, saves it to the session,
// and returns the raw RGBA image.
func (c *Controller) captureFrame() (*image.RGBA, error) {
	img, err := c.Capturer.CaptureActiveWindow()
	if err != nil {
		return nil, fmt.Errorf("capturing active window: %w", err)
	}
	if _, _, err := c.Session.SaveFrame(img); err != nil {
		return nil, fmt.Errorf("saving frame: %w", err)
	}
	c.framesCaptured++
	rgba := stitch.ToRGBA(img)
	debug.Logf("auto", "frame %d captured (raw size: %dx%d)",
		c.framesCaptured-1, rgba.Bounds().Dx(), rgba.Bounds().Dy())
	return rgba, nil
}

// calibrate runs static-edge detection on the first two frames to measure
// fixed UI (navbar, status bar) and caches both the effective viewport
// height and the edge sizes for use when cropping during overlap analysis.
func (c *Controller) calibrate(a, b *image.RGBA) {
	c.staticEdges = stitch.DetectStaticEdges([]*image.RGBA{a, b})
	h := a.Bounds().Dy() - c.staticEdges.TopRows - c.staticEdges.BottomRows
	if h <= 0 {
		h = a.Bounds().Dy()
	}
	c.viewportH = h
	debug.Logf("auto", "viewport calibrated: height=%dpx (top_static=%dpx bottom_static=%dpx)",
		c.viewportH, c.staticEdges.TopRows, c.staticEdges.BottomRows)
}

// cropForAnalysis removes the cached static edges from a frame before
// passing it to FindOverlap — mirrors what Stitch does internally so the
// controller's overlap results are consistent with the final stitch output.
// Returns the original frame if calibration hasn't run yet.
func (c *Controller) cropForAnalysis(img *image.RGBA) *image.RGBA {
	if c.viewportH == 0 {
		return img // not yet calibrated; first scroll uses full height
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	top := c.staticEdges.TopRows
	bottom := c.staticEdges.BottomRows
	newH := h - top - bottom
	if newH <= 0 {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, w, newH))
	for y := top; y < h-bottom; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y-top, img.At(x, y))
		}
	}
	return out
}

// analyze compares the previous frame with the current one using
// static-edge-cropped images, consistent with how Stitch works.
func (c *Controller) analyze(prevCropped, currentCropped *image.RGBA) stitch.OverlapResult {
	result := stitch.FindOverlap(prevCropped, currentCropped)
	return result
}

// updateStagnation updates the stagnation counters and reports whether
// either limit has been reached.
//
// Two independent signals are tracked:
//
//  1. Small-advance stagnation: a confident match was found but AddedPx is
//     below the effective minimum. The minimum is computed as the larger of
//     the configured MinimumAdvancePx and 10% of the calibrated viewport
//     height — this makes the threshold scale with the display (a fixed
//     16px is meaninglessly small on a 1500px HiDPI viewport where the
//     page bottom shows ~127px of consistent "advance" without actually
//     moving).
//
//  2. No-match stagnation: FindOverlap couldn't find a confident overlap.
//     This happens when frames are nearly identical (page is stuck at
//     bottom and all offsets score equally, failing the margin check) or
//     when dynamic content (ads, spinners) disrupts matching. Tracked
//     with a separate counter at twice the normal limit so occasional
//     jitter doesn't trigger a false stop.
func (c *Controller) updateStagnation(match stitch.OverlapResult) bool {
	if match.OverlapPx == 0 {
		// No confident match — don't affect the regular stagnation counter.
		// Use a separate, higher-limit counter so sustained no-matches
		// (e.g. identical frames at page bottom) eventually stop the session.
		c.noMatchStagnation++
		noMatchLimit := c.Config.StagnationLimit * 2
		debug.Logf("auto", "  overlap: no confident match (score=%.1f%%) — no-match stagnation: %d/%d",
			match.MatchScore*100, c.noMatchStagnation, noMatchLimit)
		return c.noMatchStagnation >= noMatchLimit
	}

	// A confident match resets the no-match counter.
	c.noMatchStagnation = 0

	// Effective minimum advance: at least 10% of the calibrated viewport so
	// the threshold scales with screen size rather than being a fixed pixel
	// count that's too small on HiDPI displays.
	effectiveMin := c.Config.MinimumAdvancePx
	if c.viewportH > 0 {
		if vMin := c.viewportH / 10; vMin > effectiveMin {
			effectiveMin = vMin
		}
	}

	if match.AddedPx < effectiveMin {
		c.stagnation++
	} else {
		c.stagnation = 0
	}

	debug.Logf("auto", "  overlap: %dpx, added: %dpx (min=%dpx), score: %.1f%% — stagnation: %d/%d",
		match.OverlapPx, match.AddedPx, effectiveMin, match.MatchScore*100,
		c.stagnation, c.Config.StagnationLimit)

	return c.stagnation >= c.Config.StagnationLimit
}

// scroll requests approximately one scrollable-viewport of downward movement.
func (c *Controller) scroll() error {
	h := c.viewportH
	if h <= 0 {
		// Not yet calibrated; use the previous frame height as a proxy.
		if c.previous != nil {
			h = c.previous.Bounds().Dy()
		}
	}
	amount := int(float64(h) * c.Config.ScrollFraction)
	debug.Logf("auto", "scrolling down %dpx (viewport=%dpx fraction=%.2f)",
		amount, h, c.Config.ScrollFraction)
	return c.Scroller.ScrollDown(amount)
}

// Run performs an automatic capture session.
//
// The caller must call sess.EnsureFresh() before Run.
func (c *Controller) Run() (Result, error) {
	debug.Logf("auto", "starting auto-capture session...")

	first, err := c.captureFrame()
	if err != nil {
		debug.Logf("auto", "initial capture failed: %v", err)
		return Result{FramesCaptured: c.framesCaptured, StopReason: StopError}, err
	}
	c.previous = first
	prevCropped := first // crop updated after calibration

	for c.framesCaptured < c.Config.MaxFrames {

		if err := c.scroll(); err != nil {
			debug.Logf("auto", "scrolling failed: %v", err)
			return Result{FramesCaptured: c.framesCaptured, StopReason: StopError}, err
		}

		time.Sleep(c.Config.Delay)

		current, err := c.captureFrame()
		if err != nil {
			debug.Logf("auto", "subsequent capture failed: %v", err)
			return Result{FramesCaptured: c.framesCaptured, StopReason: StopError}, err
		}

		// Calibrate once after the second frame.
		if c.viewportH == 0 {
			c.calibrate(first, current)
			// Re-crop first frame now that we have edge data.
			prevCropped = c.cropForAnalysis(first)
		}

		currentCropped := c.cropForAnalysis(current)
		match := c.analyze(prevCropped, currentCropped)

		if c.updateStagnation(match) {
			debug.Logf("auto", "stagnation limit reached, stopping run")
			return Result{FramesCaptured: c.framesCaptured, StopReason: StopStagnation}, nil
		}

		c.previous = current
		prevCropped = currentCropped
	}

	debug.Logf("auto", "max frame limit reached (%d), stopping run", c.Config.MaxFrames)
	return Result{FramesCaptured: c.framesCaptured, StopReason: StopMaxFrames}, nil
}

package stitch

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"scrollshot/internal/testutil"
)

// failf logs the full context needed to diagnose a failure without
// access to the machine that produced it — this matters specifically
// for CI/cloud runs, where there's no way to re-run the test
// interactively or inspect intermediate state by hand.
func failf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	t.Fatalf(format, args...)
}

// --- Stage 0: dimension validation ---

func TestStitch_DimensionMismatch_Rejected(t *testing.T) {
	// Guards against: a capture-backend switch, window resize, or
	// DPI/monitor change mid-session used to be silently cropped to a
	// common width and compared misaligned, producing corrupted output
	// with no warning. This must now be a hard, immediate error.
	f1 := ToRGBA(testutil.RandomPage(1, 400, 300))
	f2 := ToRGBA(testutil.RandomPage(2, 1200, 900))

	_, _, _, err := Stitch([]*image.RGBA{f1, f2})
	if err == nil {
		failf(t, "expected ErrDimensionMismatch for frames of width 400 and 1200, got nil error")
	}
	var mismatch *ErrDimensionMismatch
	if !asDimensionMismatch(err, &mismatch) {
		failf(t, "expected error type *ErrDimensionMismatch, got %T: %v", err, err)
	}
	if mismatch.ExpectedWidth != 400 || mismatch.ActualWidth != 1200 {
		failf(t, "error reported wrong widths: expected=%d actual=%d (want expected=400 actual=1200)",
			mismatch.ExpectedWidth, mismatch.ActualWidth)
	}
	t.Logf("correctly rejected mismatched frames: %v", err)
}

func asDimensionMismatch(err error, out **ErrDimensionMismatch) bool {
	m, ok := err.(*ErrDimensionMismatch)
	if ok {
		*out = m
	}
	return ok
}

func TestStitch_SameWidth_DifferentHeight_Allowed(t *testing.T) {
	// Only width is validated, not height — frames legitimately differ
	// in height (the last capture in a session is often a shorter
	// partial scroll). This must NOT be rejected.
	f1 := ToRGBA(testutil.RandomPage(1, 400, 500))
	f2 := ToRGBA(testutil.RandomPage(2, 400, 200))

	_, _, _, err := Stitch([]*image.RGBA{f1, f2})
	if err != nil {
		failf(t, "frames of equal width but different height should be allowed, got error: %v", err)
	}
}

// --- Core overlap matching ---

func TestStitch_BasicOverlap_ExactMatch(t *testing.T) {
	// Baseline regression case: high-entropy content, three frames with
	// a known, exact 150px overlap between each consecutive pair. If
	// this ever stops being exact, something fundamental broke.
	full := testutil.RandomPage(1, 400, 1200)
	frames := []*image.RGBA{
		ToRGBA(testutil.Crop(full, 0, 500)),
		ToRGBA(testutil.Crop(full, 350, 850)),
		ToRGBA(testutil.Crop(full, 700, 1200)),
	}

	result, log, _, err := Stitch(frames)
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if result == nil {
		failf(t, "expected non-nil result")
	}
	for i, want := range []int{0, 150, 150} {
		if i == 0 {
			continue
		}
		if log[i].OverlapPx != want {
			failf(t, "frame %d: expected exact %dpx overlap, got %dpx (full log: %+v)", i, want, log[i].OverlapPx, log)
		}
	}
	wantHeight := 500 + 350 + 350
	if result.Bounds().Dy() != wantHeight {
		failf(t, "expected final height %dpx, got %dpx", wantHeight, result.Bounds().Dy())
	}
	t.Logf("exact match confirmed: overlaps=%dpx,%dpx final_height=%dpx", log[1].OverlapPx, log[2].OverlapPx, result.Bounds().Dy())
}

func TestStitch_GenuinelyUnrelatedFrames_Rejected(t *testing.T) {
	// Two frames with no real relationship must be rejected (0 overlap),
	// not confidently matched at some arbitrary offset.
	f1 := ToRGBA(testutil.RandomPage(100, 400, 600))
	f2 := ToRGBA(testutil.RandomPage(200, 400, 600))

	_, log, _, err := Stitch([]*image.RGBA{f1, f2})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if log[1].OverlapPx != 0 {
		failf(t, "expected rejection (0px overlap) for genuinely unrelated frames, got %dpx overlap at score %.1f%%",
			log[1].OverlapPx, log[1].MatchScore*100)
	}
	if !log[1].LowConfident {
		failf(t, "expected LowConfident=true for a rejected match")
	}
	t.Logf("correctly rejected unrelated frames: score=%.1f%%", log[1].MatchScore*100)
}

func TestStitch_RealOverlapWithLocalizedNoise_StillFound(t *testing.T) {
	// A real overlap containing a small patch of genuinely different
	// pixels (simulating a blinking cursor, a live timestamp, a hover
	// state) must still be found — the match score is tolerance-based,
	// not a raw mean, specifically so small localized noise doesn't
	// sink an otherwise-correct match.
	full := testutil.RandomPage(55, 400, 1200)
	f1 := testutil.Crop(full, 0, 500)
	f2 := testutil.Crop(full, 350, 850)
	for y := 20; y < 40; y++ {
		for x := 100; x < 120; x++ {
			f2.Set(x, y, color.RGBA{200, 50, 50, 255})
		}
	}

	frames := []*image.RGBA{ToRGBA(f1), ToRGBA(f2)}
	_, log, _, err := Stitch(frames)
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if log[1].OverlapPx != 150 {
		failf(t, "expected exact 150px overlap despite localized noise patch, got %dpx (score %.1f%%)",
			log[1].OverlapPx, log[1].MatchScore*100)
	}
}

// --- Background-domination bugs (Stage 2/3) ---

func TestStitch_BackgroundDominatedContent_IdenticalFrames_FindsFullMatch(t *testing.T) {
	// Guards against the core background-domination bug: on content
	// where 95%+ of pixels are a uniform background (a dark code
	// editor), a naive percentage-match score is dominated by trivial
	// background-vs-background agreement regardless of true alignment.
	// Two IDENTICAL frames of such content must still correctly resolve
	// to a full match — this was the exact case that originally
	// produced a false REJECTION (paradoxically) once background
	// exclusion was first added without the "fall back to all columns
	// for uniform rows" refinement.
	page := testutil.CodeEditorPage(7, 1800, 1200)
	f1 := ToRGBA(testutil.Crop(page, 0, 800))
	f2 := ToRGBA(testutil.Crop(page, 0, 800))

	result, log, static, err := Stitch([]*image.RGBA{f1, f2})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	// Known limitation (see docs/known-limitations.md): fully-identical
	// frames create a genuine ambiguity for static-edge detection, which
	// can crop a bounded amount from each edge (capped by
	// maxStaticFraction). We assert the crop stays within that documented
	// bound rather than asserting zero crop, since zero crop isn't
	// achievable with only two identical frames and no session-level
	// context — see docs/known-limitations.md before "fixing" this by
	// loosening the assertion instead of improving the algorithm.
	maxExpectedCropPerEdge := int(float64(800) * maxStaticFraction)
	if static.TopRows > maxExpectedCropPerEdge || static.BottomRows > maxExpectedCropPerEdge {
		failf(t, "static crop exceeded documented cap: top=%d bottom=%d, cap=%d per edge",
			static.TopRows, static.BottomRows, maxExpectedCropPerEdge)
	}
	if log[1].MatchScore < 0.99 {
		failf(t, "identical content should score near-100%%, got %.1f%%", log[1].MatchScore*100)
	}
	t.Logf("identical-frame ambiguity within documented bound: top_crop=%d bottom_crop=%d (cap=%d), final_height=%d",
		static.TopRows, static.BottomRows, maxExpectedCropPerEdge, result.Bounds().Dy())
}

func TestStitch_RepetitiveBackgroundHeavyContent_SmallTrueOverlap_KnownImprecision(t *testing.T) {
	// This is the "Test D" scenario from manual development testing,
	// documented as a known limitation in docs/known-limitations.md:
	// "Small residual imprecision on extreme low-signal repetitive
	// content." True overlap is 60px; the matcher currently finds 50px.
	//
	// This test intentionally asserts the CURRENT, documented, imperfect
	// behavior (not the ideal 60px) — its job is to catch any further
	// drift in either direction, not to silently pass by asserting
	// whatever the code happens to do. If this starts finding exactly
	// 60px, that's a genuine improvement — update this test to require
	// the exact match and remove the known-limitations.md entry. If it
	// finds something other than 50px, that's worth investigating before
	// just updating the number.
	page := testutil.CodeEditorPage(7, 1800, 1200)
	f1 := ToRGBA(testutil.Crop(page, 0, 800))
	f2 := ToRGBA(testutil.Crop(page, 740, 1200)) // true overlap = 60px

	_, log, _, err := Stitch([]*image.RGBA{f1, f2})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	const trueOverlap = 60
	const currentDocumentedResult = 50
	if log[1].OverlapPx != currentDocumentedResult {
		failf(t, "expected the documented current result of %dpx (true overlap is %dpx — see "+
			"docs/known-limitations.md 'Small residual imprecision on extreme low-signal repetitive content'), "+
			"got %dpx instead (score %.1f%%). If this is now exact (%dpx), that's an improvement — "+
			"update this test and the known-limitations doc. If it's neither value, investigate before updating.",
			currentDocumentedResult, trueOverlap, log[1].OverlapPx, log[1].MatchScore*100, trueOverlap)
	}
	t.Logf("known limitation confirmed unchanged: true=%dpx found=%dpx (documented gap tracked)", trueOverlap, log[1].OverlapPx)
}

// --- Anti-aliasing / rendering noise tolerance ---

func TestStitch_RealisticRenderingJitter_ExactMatch(t *testing.T) {
	// Guards against: two independent captures of the same logical text
	// content are never byte-identical (font anti-aliasing/hinting
	// varies between renders). pointTolerance was originally calibrated
	// on clean synthetic data and was too strict for this, causing real
	// matches to score 83-88% and get rejected. This test's jitter
	// amount (20) was specifically chosen to reproduce that real-world
	// score range — see docs/stitching.md for the calibration story.
	full := testutil.RenderedTextPage(77, 1800, 1600)
	f1 := testutil.Jitter(testutil.Crop(full, 0, 800), 1, 20)
	f2 := testutil.Jitter(testutil.Crop(full, 600, 1400), 2, 20)

	_, log, _, err := Stitch([]*image.RGBA{ToRGBA(f1), ToRGBA(f2)})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if log[1].OverlapPx != 200 {
		failf(t, "expected exact 200px overlap despite realistic AA jitter, got %dpx (score %.1f%%) — "+
			"if this regresses, pointTolerance is likely too strict again",
			log[1].OverlapPx, log[1].MatchScore*100)
	}
}

// --- Static UI detection (Stage 1) ---

func TestStitch_StickyTopNav_CroppedAndOverlapCorrect(t *testing.T) {
	// Guards against the real, user-reported sweetgreen.com bug: a
	// sticky top nav present unchanged in every frame was getting
	// duplicated in the output instead of recognized as fixed UI.
	const navHeight = 40
	nav := testutil.SolidBlock(400, navHeight, color.RGBA{20, 60, 40, 255})
	content := testutil.RandomPage(3, 400, 1200)

	windows := [][2]int{{0, 460}, {350, 810}, {700, 1160}}
	frames := make([]*image.RGBA, len(windows))
	for i, w := range windows {
		frame := image.NewRGBA(image.Rect(0, 0, 400, 500))
		testutil.Paste(frame, nav, 0)
		testutil.Paste(frame, testutil.Crop(content, w[0], w[1]), navHeight)
		frames[i] = frame
	}

	result, log, static, err := Stitch(frames)
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if static.TopRows != navHeight {
		failf(t, "expected exactly %dpx detected as static top UI, got %dpx", navHeight, static.TopRows)
	}
	if static.BottomRows != 0 {
		failf(t, "expected 0px static bottom (no fixed bottom UI in this scenario), got %dpx", static.BottomRows)
	}
	for i, want := range []int{0, 110, 110} {
		if i == 0 {
			continue
		}
		if log[i].OverlapPx != want {
			failf(t, "frame %d: expected exact %dpx content overlap (after nav crop), got %dpx", i, want, log[i].OverlapPx)
		}
	}
	// The nav's color must not appear anywhere in the output — it should
	// be cropped from every frame, including the first (see
	// docs/known-limitations.md for why this is the chosen default).
	navColorRows := countRowsMatchingColor(result, color.RGBA{20, 60, 40, 255})
	if navColorRows != 0 {
		failf(t, "nav color found in %d rows of final output — expected 0 (nav should be fully cropped, not shown once)", navColorRows)
	}
	t.Logf("sticky nav correctly cropped and excluded from output: detected=%dpx final_height=%dpx", static.TopRows, result.Bounds().Dy())
}

func TestStitch_FixedBottomStatusBar_CroppedAndOverlapCorrect(t *testing.T) {
	// Guards against the real, user-reported VS Code bug: a fixed
	// bottom status bar (git branch, language mode, etc.) present in
	// every frame was causing false full-height matches, since the
	// matcher's reference strip landed entirely on the status bar.
	const statusHeight = 30
	status := testutil.SolidBlock(400, statusHeight, color.RGBA{10, 90, 160, 255})
	content := testutil.RandomPage(9, 400, 1000)

	windows := [][2]int{{0, 400}, {370, 770}, {740, 1000}}
	frames := make([]*image.RGBA, len(windows))
	for i, w := range windows {
		part := testutil.Crop(content, w[0], w[1])
		frame := image.NewRGBA(image.Rect(0, 0, 400, part.Bounds().Dy()+statusHeight))
		testutil.Paste(frame, part, 0)
		testutil.Paste(frame, status, part.Bounds().Dy())
		frames[i] = frame
	}

	_, log, static, err := Stitch(frames)
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if static.BottomRows != statusHeight {
		failf(t, "expected exactly %dpx detected as static bottom UI, got %dpx", statusHeight, static.BottomRows)
	}
	if static.TopRows != 0 {
		failf(t, "expected 0px static top, got %dpx", static.TopRows)
	}
	for i, want := range []int{0, 30, 30} {
		if i == 0 {
			continue
		}
		if log[i].OverlapPx != want {
			failf(t, "frame %d: expected exact %dpx content overlap (after status bar crop), got %dpx", i, want, log[i].OverlapPx)
		}
	}
}

func TestStitch_TopNavAndBottomStatusBar_BothDetectedAndCropped(t *testing.T) {
	// Combined case: both a sticky header AND a fixed footer present at
	// once, exercising DetectStaticEdges' top and bottom scans
	// independently in the same session.
	const navHeight = 40
	const statusHeight = 30
	nav := testutil.SolidBlock(400, navHeight, color.RGBA{20, 60, 40, 255})
	status := testutil.SolidBlock(400, statusHeight, color.RGBA{10, 90, 160, 255})
	content := testutil.RandomPage(21, 400, 1200)

	windows := [][2]int{{0, 400}, {350, 750}, {700, 1100}}
	frames := make([]*image.RGBA, len(windows))
	for i, w := range windows {
		part := testutil.Crop(content, w[0], w[1])
		frame := image.NewRGBA(image.Rect(0, 0, 400, navHeight+part.Bounds().Dy()+statusHeight))
		testutil.Paste(frame, nav, 0)
		testutil.Paste(frame, part, navHeight)
		testutil.Paste(frame, status, navHeight+part.Bounds().Dy())
		frames[i] = frame
	}

	_, log, static, err := Stitch(frames)
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if static.TopRows != navHeight || static.BottomRows != statusHeight {
		failf(t, "expected top=%d bottom=%d, got top=%d bottom=%d", navHeight, statusHeight, static.TopRows, static.BottomRows)
	}
	for i, want := range []int{0, 50, 50} {
		if i == 0 {
			continue
		}
		if log[i].OverlapPx != want {
			failf(t, "frame %d: expected exact %dpx content overlap, got %dpx", i, want, log[i].OverlapPx)
		}
	}
}

func TestStitch_NoStaticUI_NoFalsePositiveCrop(t *testing.T) {
	// Guards the inverse case: ordinary scrolling content with no fixed
	// UI at all must NOT have anything cropped as "static" — false
	// positives here silently destroy real content.
	full := testutil.RandomPage(1, 400, 1200)
	frames := []*image.RGBA{
		ToRGBA(testutil.Crop(full, 0, 500)),
		ToRGBA(testutil.Crop(full, 350, 850)),
		ToRGBA(testutil.Crop(full, 700, 1200)),
	}

	_, _, static, err := Stitch(frames)
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if static.TopRows != 0 || static.BottomRows != 0 {
		failf(t, "expected no static UI detected on ordinary high-entropy scrolling content, got top=%d bottom=%d",
			static.TopRows, static.BottomRows)
	}
}

// --- Confidence margin / high-confidence override (Stage 3) ---

func TestStitch_RepeatedCodeBlocks_HighConfidenceOverrideAccepts(t *testing.T) {
	// Guards against the real, user-reported bug: on structurally
	// repetitive real code, a distant rival can score high enough that
	// the margin check rejects an otherwise-correct 99%+ match. This
	// reproduces the exact scenario (repeating code-block structure,
	// small 87px true overlap) that originally produced rejected
	// 99-100% matches in a real VS Code capture.
	page := testutil.RepeatingBlockPage(500, 1800, 3000, 90)
	f1 := ToRGBA(testutil.Crop(page, 0, 1533))
	f2 := ToRGBA(testutil.Crop(page, 1446, 2979)) // true overlap = 87px

	_, log, _, err := Stitch([]*image.RGBA{f1, f2})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if log[1].OverlapPx != 87 {
		failf(t, "expected exact 87px overlap to be ACCEPTED via the high-confidence override, got %dpx (score %.1f%%) — "+
			"if this regresses, check highConfidenceOverride and minMargin",
			log[1].OverlapPx, log[1].MatchScore*100)
	}
}

func TestStitch_RepetitiveJitteredContent_FalsePositiveStillRejected(t *testing.T) {
	// Guards the flip side of the override: it must NOT blindly accept
	// any high-scoring match. This reproduces the adversarial case found
	// during calibration — repeating block content combined with
	// realistic rendering jitter produced a genuinely WRONG offset
	// scoring 97.1%. highConfidenceOverride (0.985) must sit above that
	// measured false-positive ceiling, or this test will start failing
	// (which is the point: it's a tripwire for a threshold that's
	// drifted too low).
	page := testutil.CodeEditorPage(77, 1800, 1600)
	f1 := testutil.Jitter(testutil.Crop(page, 0, 800), 1, 20)
	f2 := testutil.Jitter(testutil.Crop(page, 600, 1400), 2, 20) // true overlap = 200px

	_, log, _, err := Stitch([]*image.RGBA{ToRGBA(f1), ToRGBA(f2)})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	// The true overlap (200px) may or may not be found depending on
	// tolerance/margin tuning at the time — what this test actually
	// guards is that the WRONG high-scoring offset (~674px, ~97.1%
	// score, measured during calibration) must never be accepted.
	if log[1].OverlapPx > 250 {
		failf(t, "accepted a suspiciously large overlap (%dpx, score %.1f%%) on adversarial repetitive+jittered "+
			"content — this is the exact shape of false positive highConfidenceOverride's calibration was meant to "+
			"prevent; check whether the threshold has drifted below a real false-positive's score",
			log[1].OverlapPx, log[1].MatchScore*100)
	}
	t.Logf("adversarial case handled: overlap=%dpx score=%.1f%%", log[1].OverlapPx, log[1].MatchScore*100)
}

// --- Multi-frame session behavior ---

func TestStitch_EmptyFrameList_ReturnsNilWithoutError(t *testing.T) {
	result, log, static, err := Stitch(nil)
	if err != nil {
		failf(t, "expected nil error for empty frame list, got: %v", err)
	}
	if result != nil || log != nil {
		failf(t, "expected nil result and log for empty frame list, got result=%v log=%v", result, log)
	}
	_ = static
}

func TestStitch_SingleFrame_ReturnedUnchanged(t *testing.T) {
	f := ToRGBA(testutil.RandomPage(1, 400, 300))
	result, log, _, err := Stitch([]*image.RGBA{f})
	if err != nil {
		failf(t, "unexpected error: %v", err)
	}
	if result.Bounds() != f.Bounds() {
		failf(t, "expected single frame's bounds unchanged, got %v (from %v)", result.Bounds(), f.Bounds())
	}
	if len(log) != 1 || log[0].Index != 0 {
		failf(t, "expected exactly one log entry for a single frame, got %+v", log)
	}
}

// countRowsMatchingColor counts how many rows of img have their
// leftmost pixel matching c within a small tolerance — used to verify a
// solid-color block (like a nav bar) does or doesn't appear in output.
func countRowsMatchingColor(img *image.RGBA, c color.RGBA) int {
	b := img.Bounds()
	count := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		r, g, bl, _ := img.At(b.Min.X, y).RGBA()
		cr, cg, cb, _ := c.RGBA()
		if absDiffU32(r, cr) < 2000 && absDiffU32(g, cg) < 2000 && absDiffU32(bl, cb) < 2000 {
			count++
		}
	}
	return count
}

func absDiffU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestMain provides a single point to add setup/teardown as the suite
// grows (e.g. writing failure artifacts to disk for CI log collection),
// without every test needing to know about it.
func TestMain(m *testing.M) {
	code := m.Run()
	if code != 0 {
		fmt.Println("stitch package tests failed — see individual test output above for the specific scenario and measured scores")
	}
	os.Exit(code)
}

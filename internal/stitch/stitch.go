// Package stitch contains the scrolling-screenshot stitching engine:
// given a sequence of overlapping frame images, it finds where each
// consecutive pair overlaps and glues them into one tall image.
//
// This package is deliberately independent of how frames were captured —
// it just takes image.Image in and produces image.RGBA out — so it works
// unchanged regardless of which capture backend produced the frames.
//
// Matching approach (see project notes for the full edge-case catalog
// this is designed against):
//   - Stage 0: frames must share identical dimensions before matching
//     even starts. A silent width-crop previously let mismatched frames
//     (different capture backend, window resize, DPI change mid-session)
//     get compared as if they lined up, producing garbage output with no
//     warning. Now it's a hard, explicit error instead.
//   - Stage 3: similarity is measured as the percentage of sampled
//     points that agree within a per-pixel tolerance, not a raw mean
//     difference. A mean is fragile — a handful of genuinely different
//     pixels (a blinking cursor, a live timestamp, a hover state) can
//     swing it a lot even when the vast majority of the region matches
//     perfectly. A tolerant match-percentage absorbs small localized
//     noise without needing to special-case what caused it.
//   - Stage 4: a match is only accepted if it's both individually
//     confident (high match percentage on its own) AND meaningfully
//     better than the next-best alternative offset. Low-entropy content
//     (dark-theme code editors, large flat-color backgrounds) can make
//     many wrong offsets score deceptively well; requiring the winner to
//     clearly stand out — not just edge out a crowded field — is what
//     catches that instead of confidently guessing wrong.
package stitch

import (
	"fmt"
	"image"
)

// FrameResult reports what happened when stitching one frame onto the
// growing image, for logging/debugging (e.g. spotting a 0px-overlap
// frame that might indicate a missed match rather than a real jump).
type FrameResult struct {
	Index        int
	OverlapPx    int
	AddedPx      int
	MatchScore   float64 // 0..1, fraction of sampled points that agreed within tolerance
	LowConfident bool    // true if no match cleared both the score and margin bars
}

// ToRGBA converts any image.Image into *image.RGBA, which the rest of
// this package operates on for direct pixel access.
func ToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}

const (
	// stripHeight is the thickness of the reference strip taken from the
	// bottom of the previous frame when searching for the overlap point.
	stripHeight = 20

	// pointTolerance is how much a single sampled pixel's combined RGB
	// difference can be and still count as "matching" — absorbs small
	// noise (compression artifacts, anti-aliasing/font-hinting jitter
	// between two independently-rendered captures of the same content)
	// without requiring byte-exact equality.
	pointTolerance = 90.0

	// staticEdgeTolerance is the (stricter) tolerance used only by
	// DetectStaticEdges. A false positive there destructively crops real
	// content before matching ever gets a chance to run on it — unlike
	// overlap-matching, where a rejected match just means a retry, a
	// wrongly-detected "static" region silently eats real scrolled
	// content. Deliberately kept tighter than pointTolerance so that
	// loosening overlap-matching for anti-aliasing noise doesn't also
	// make static-region detection trigger-happy on ordinary content
	// that happens to look similar between just two frames.
	staticEdgeTolerance = 60.0

	// minMatchScore is the minimum fraction of sampled points that must
	// agree within tolerance for a candidate offset to be accepted at
	// all, regardless of how it compares to other candidates.
	minMatchScore = 0.90

	// minMargin is how much better the best candidate's match score must
	// be than the next-best *distant* candidate's score for the match to
	// be trusted. Content with a lot of repeated structure (code editors,
	// flat backgrounds) can make several wrong offsets score nearly as
	// well as the right one; this catches that ambiguity and rejects it
	// instead of guessing.
	minMargin = 0.06

	// highConfidenceOverride: a candidate scoring at or above this bypasses
	// the margin check entirely. Calibrated against measured evidence, not
	// guessed: on adversarial content combining structural repetition
	// (repeated code-like blocks) with realistic rendering noise, a
	// genuinely WRONG offset was found to score as high as 97.1% — so the
	// override sits well above that measured false-positive ceiling, while
	// staying below a true match's measured 99.7% on the same content,
	// preserving margin on both sides.
	highConfidenceOverride = 0.985

	// marginExclusionRadius: candidates within this many pixels of the
	// best offset are *not* used as the "next-best" comparison, since
	// neighboring offsets naturally score similarly to a true match
	// (the content barely shifts) and comparing against them would make
	// every real match look falsely ambiguous.
	marginExclusionRadius = stripHeight * 2

	// bgTolerance: how close a pixel must be to the dominant/background
	// color to be excluded from scoring. Uniform backgrounds (flat page
	// color, dark editor theme) can dominate a region so heavily that a
	// raw percentage-match score stays high (~95%+) regardless of
	// whether the *distinctive* content (text, images, UI elements)
	// actually lines up — background-matching-background is trivially
	// true at almost any offset and tells us nothing about alignment.
	// Excluding it keeps the score focused on the part that actually
	// distinguishes a true match from a coincidental one.
	bgTolerance = 40.0

	// minInformativePoints: if fewer than this many non-background
	// points are available to compare (sparse pass) or found in the
	// dense re-check, there isn't enough distinctive content in this
	// region to trust *any* score computed from it — treated as an
	// automatic reject rather than reporting a percentage over too thin
	// a sample to mean anything.
	minInformativePoints = 25
)

// ErrDimensionMismatch is returned by Stitch when frames don't share the
// same width — comparing them would silently misalign every column.
type ErrDimensionMismatch struct {
	FrameIndex    int
	ExpectedWidth int
	ActualWidth   int
}

func (e *ErrDimensionMismatch) Error() string {
	return fmt.Sprintf(
		"frame %d has width %dpx, but earlier frames are %dpx — frames must be captured at the same size (check for a capture-backend switch, window resize, or DPI/monitor change mid-session)",
		e.FrameIndex, e.ActualWidth, e.ExpectedWidth,
	)
}

// dominantColor finds the most common color in the bottom `height` rows
// of img, quantized coarsely to group near-identical shades (anti-
// aliasing, compression noise) into the same bucket. This is the
// "background" that gets excluded from scoring — the content that's
// present regardless of scroll position tells us nothing about
// alignment, so points matching it don't count as evidence either way.
func dominantColor(img *image.RGBA, height int) (r, g, b uint32) {
	bounds := img.Bounds()
	h := bounds.Dy()
	if height > h {
		height = h
	}
	startY := h - height

	type key struct{ r, g, b uint8 }
	counts := make(map[key]int)
	for y := startY; y < h; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pr, pg, pb, _ := img.At(x, y).RGBA()
			// quantize to 16 buckets per channel to group near-identical shades
			k := key{uint8(pr >> 12), uint8(pg >> 12), uint8(pb >> 12)}
			counts[k]++
		}
	}
	var best key
	bestCount := -1
	for k, c := range counts {
		if c > bestCount {
			bestCount = c
			best = k
		}
	}
	return uint32(best.r) << 12, uint32(best.g) << 12, uint32(best.b) << 12
}

func isBackground(r, g, b, bgR, bgG, bgB uint32) bool {
	return absDiff(r, bgR)+absDiff(g, bgG)+absDiff(b, bgB) <= bgTolerance*257
}

// candidate is one offset considered during the search, kept around so
// the margin/confidence check can look at more than just the winner.
type candidate struct {
	offset int
	score  float64
}

// scoreOffset measures what fraction of *non-background* sampled points
// in the reference strip (bottom `stripHeight` rows of `top`) match the
// corresponding points in `bottom` starting at `offset`, within
// pointTolerance. Background points (matching bgR/bgG/bgB) are excluded
// entirely — they'd trivially "match" at nearly any offset and dilute
// the score with points that carry no real alignment information.
// Returns (score, informativePoints) — callers should treat a score as
// untrustworthy if informativePoints is below minInformativePoints.
func scoreOffset(top, bottom *image.RGBA, cols []int, offset int, bgR, bgG, bgB uint32) (float64, int) {
	hTop := top.Bounds().Dy()
	var matched, total int
	for _, x := range cols {
		for k := 0; k < stripHeight; k++ {
			tr, tg, tb, _ := top.At(x, hTop-stripHeight+k).RGBA()
			if isBackground(tr, tg, tb, bgR, bgG, bgB) {
				continue
			}
			br, bg, bb, _ := bottom.At(x, offset+k).RGBA()
			diff := absDiff(tr, br) + absDiff(tg, bg) + absDiff(tb, bb)
			// RGBA() returns 16-bit-scaled values; scale tolerance to match
			if diff <= pointTolerance*257 {
				matched++
			}
			total++
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(matched) / float64(total), total
}

// scoreOffsetDense is like scoreOffset but checks every column (not a
// sparse sample) and a taller verification window — used only on the
// shortlisted top candidates from the cheap sparse pass, to get an
// accurate score before the confidence/margin decision. Sparse sampling
// is fine for ranking every possible offset cheaply, but content with a
// dominant uniform background and only sparse distinguishing detail
// (e.g. a dark-theme code editor) can make sparse sampling miss the
// detail that actually separates a true match from a coincidental one.
func scoreOffsetDense(top, bottom *image.RGBA, offset int, bgR, bgG, bgB uint32) (float64, int) {
	hTop := top.Bounds().Dy()
	w := top.Bounds().Dx()

	verifyHeight := stripHeight * 3
	if verifyHeight > hTop {
		verifyHeight = hTop
	}
	// bottom's window start (offset - (verifyHeight - stripHeight)) must
	// stay non-negative
	if verifyHeight > offset+stripHeight {
		verifyHeight = offset + stripHeight
	}
	if verifyHeight <= 0 {
		return 0, 0
	}

	topStart := hTop - verifyHeight
	bottomStart := offset - (verifyHeight - stripHeight)

	var matched, total int
	for x := 0; x < w; x++ {
		for k := 0; k < verifyHeight; k++ {
			tr, tg, tb, _ := top.At(x, topStart+k).RGBA()
			if isBackground(tr, tg, tb, bgR, bgG, bgB) {
				continue
			}
			br, bg, bb, _ := bottom.At(x, bottomStart+k).RGBA()
			diff := absDiff(tr, br) + absDiff(tg, bg) + absDiff(tb, bb)
			if diff <= pointTolerance*257 {
				matched++
			}
			total++
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(matched) / float64(total), total
}

// FindOverlap locates how many rows at the bottom of `top` are duplicated
// at the top of `bottom` (the region scrolled past but re-captured).
// Returns the overlap size and the winning match score. Returns
// (0, score) if no candidate is confident and distinctive enough to
// trust — score in that case is the best dense score found, useful for
// debugging/logging even on rejection.
//
// Two-pass approach: a cheap sparse-column scan ranks every possible
// offset fast, then only the best offset and its best *distant* rival
// get a full, dense (every column, taller window) re-check before the
// accept/reject decision. Both passes exclude points matching the
// region's dominant/background color — on content dominated by a flat
// background (a plain page color, a dark editor theme), background-vs-
// background trivially "matches" at nearly any offset and would swamp
// the score with information-free agreement, masking whether the
// content that actually distinguishes a true match from a wrong one
// lines up at all.
func FindOverlap(top, bottom *image.RGBA) (int, float64) {
	hTop := top.Bounds().Dy()
	hBot := bottom.Bounds().Dy()
	w := top.Bounds().Dx()

	if hTop < stripHeight || hBot < stripHeight {
		return 0, 0
	}

	bgR, bgG, bgB := dominantColor(top, stripHeight*3)

	numCols := 150
	if w < numCols {
		numCols = w
	}
	step := w / numCols
	if step < 1 {
		step = 1
	}
	var cols []int
	for x := 0; x < w; x += step {
		cols = append(cols, x)
	}

	maxOffset := hBot - stripHeight
	if maxOffset > hTop-stripHeight {
		maxOffset = hTop - stripHeight
	}
	if maxOffset < 0 {
		return 0, 0
	}

	// Pass 1: cheap sparse scan across every possible offset.
	var candidates []candidate
	for offset := 0; offset <= maxOffset; offset++ {
		score, informative := scoreOffset(top, bottom, cols, offset, bgR, bgG, bgB)
		if informative < minInformativePoints/5 { // sparse pass sees fewer points by construction; scale threshold down accordingly
			continue
		}
		candidates = append(candidates, candidate{offset: offset, score: score})
	}
	if len(candidates) == 0 {
		// No offset had enough distinctive content in the sparse sample at
		// all — nothing trustworthy to rank. Dense fallback below still
		// gets a chance at the naive best-guess offset in this rare case.
		return 0, 0
	}

	sparseBest := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > sparseBest.score {
			sparseBest = c
		}
	}
	sparseRival := candidate{score: -1}
	for _, c := range candidates {
		if abs(c.offset-sparseBest.offset) <= marginExclusionRadius {
			continue
		}
		if c.score > sparseRival.score {
			sparseRival = c
		}
	}

	// Pass 2: dense, full-column re-verification on just those two —
	// this is what the accept/reject decision actually relies on.
	bestScore, bestInformative := scoreOffsetDense(top, bottom, sparseBest.offset, bgR, bgG, bgB)

	// If even the dense, full-width re-check can't find enough distinctive
	// (non-background) content to compare, there's nothing to confidently
	// match against — reject rather than trust a score built on too thin
	// a sample.
	if bestInformative < minInformativePoints {
		return 0, bestScore
	}

	// Confidence bar: the winner has to be a good match on its own merits.
	if bestScore < minMatchScore {
		return 0, bestScore
	}

	// Margin bar, using dense scores for both sides of the comparison —
	// skipped for a near-perfect match. Requiring daylight over a rival
	// makes sense when the winner's own score is merely "good enough";
	// it stops making sense once the winner is already almost certainly
	// correct on its own merits. Structurally repetitive real content
	// (many similarly-indented "if err != nil" blocks in source code,
	// for instance) can produce a distant rival that also scores
	// deceptively high — but a 97%+ match is itself strong evidence,
	// independent of how close a rival happens to score.
	if bestScore < highConfidenceOverride && sparseRival.score >= 0 {
		rivalScore, _ := scoreOffsetDense(top, bottom, sparseRival.offset, bgR, bgG, bgB)
		if (bestScore - rivalScore) < minMargin {
			return 0, bestScore
		}
	}

	return stripHeight + sparseBest.offset, bestScore
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func absDiff(a, b uint32) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}

// StaticEdges reports how much of the frame's top and bottom is fixed UI
// (a sticky nav, a status bar) rather than scrollable content.
type StaticEdges struct {
	TopRows    int
	BottomRows int
}

// staticRowMatchThreshold: how much of a row's sampled columns must agree
// (within pointTolerance) for that row to count as "the same" between two
// frames when detecting fixed UI.
const staticRowMatchThreshold = 0.95

// maxStaticFraction caps how much of a frame's height can be classified
// as static UI, as a safety net against pathological cases (e.g. a
// session where every frame is genuinely identical) swallowing the
// entire image as "fixed".
const maxStaticFraction = 0.12

// DetectStaticEdges scans across ALL frames in a session at once (not
// just consecutive pairs) for rows that stay nearly identical at the
// same absolute position in every single one — the signature of fixed
// UI that doesn't scroll with the content: a sticky nav/header at the
// top, a status/tool bar at the bottom, browser chrome. A pairwise check
// could be fooled by two frames coincidentally agreeing; something
// unchanged across every frame in the whole session is a much stronger
// signal.
//
// Checks top and bottom independently, since either — or both — can be
// present (a page's sticky nav is a top case; an editor's status bar is
// a bottom case).
// minStaticDistinctiveCols: a row needs at least this many non-background
// sampled columns before DetectStaticEdges will judge it confidently.
// Without this, a row that's mostly-background would trivially "match"
// across frames (background-vs-background) regardless of whether the
// sparse real content on it is actually fixed — the same background-
// domination failure mode fixed earlier in overlap scoring, which turned
// out to affect this detector too and was only caught by testing against
// background-heavy content, not the solid-color synthetic UI used while
// building this function.
const minStaticDistinctiveCols = 15

func DetectStaticEdges(frames []*image.RGBA) StaticEdges {
	if len(frames) < 2 {
		return StaticEdges{}
	}

	w := frames[0].Bounds().Dx()
	minHeight := frames[0].Bounds().Dy()
	for _, f := range frames[1:] {
		if h := f.Bounds().Dy(); h < minHeight {
			minHeight = h
		}
	}

	bgR, bgG, bgB := dominantColor(frames[0], frames[0].Bounds().Dy())

	allCols := make([]int, w)
	for x := 0; x < w; x++ {
		allCols[x] = x
	}

	maxStatic := int(float64(minHeight) * maxStaticFraction)

	// rowStaticAcrossAll checks whether the row resolved by `pick` matches,
	// within tolerance, across every frame relative to frame 0 — using
	// `pick` so top and bottom edges (and frames of possibly-different
	// heights) share one implementation.
	//
	// Two cases, handled differently:
	//   - The row has enough non-background columns to judge on those
	//     alone: background trivially agreeing across frames proves
	//     nothing, so judging only the distinctive columns avoids a
	//     handful of real (and possibly non-matching) content getting
	//     diluted/hidden by a majority-background row scoring high
	//     overall regardless of whether that real content actually
	//     matches.
	//   - The row is essentially uniform (too few non-background columns
	//     to judge distinctively) — that's not an "uninformative" row to
	//     give up on, it's the signature of a solid-colored fixed block
	//     (e.g. a plain-colored nav bar), which IS the thing this
	//     function needs to detect. Falls back to judging all columns,
	//     since for a genuinely uniform row that's the only — and a
	//     perfectly adequate — signal available.
	rowStaticAcrossAll := func(pick func(f *image.RGBA) int) bool {
		y0 := pick(frames[0])
		var distinctiveCols []int
		for x := 0; x < w; x++ {
			r0, g0, b0, _ := frames[0].At(x, y0).RGBA()
			if !isBackground(r0, g0, b0, bgR, bgG, bgB) {
				distinctiveCols = append(distinctiveCols, x)
			}
		}
		judgeCols := distinctiveCols
		if len(judgeCols) < minStaticDistinctiveCols {
			judgeCols = allCols // fall back to all columns for uniform rows
		}
		for i := 1; i < len(frames); i++ {
			yi := pick(frames[i])
			matched := 0
			for _, x := range judgeCols {
				r0, g0, b0, _ := frames[0].At(x, y0).RGBA()
				ri, gi, bi, _ := frames[i].At(x, yi).RGBA()
				diff := absDiff(r0, ri) + absDiff(g0, gi) + absDiff(b0, bi)
				if diff <= staticEdgeTolerance*257 {
					matched++
				}
			}
			if float64(matched)/float64(len(judgeCols)) < staticRowMatchThreshold {
				return false
			}
		}
		return true
	}

	var top int
	for top = 0; top < maxStatic; top++ {
		row := top
		if !rowStaticAcrossAll(func(f *image.RGBA) int { return row }) {
			break
		}
	}

	var bottom int
	for bottom = 0; bottom < maxStatic; bottom++ {
		fromBottom := bottom
		if !rowStaticAcrossAll(func(f *image.RGBA) int { return f.Bounds().Dy() - 1 - fromBottom }) {
			break
		}
	}

	return StaticEdges{TopRows: top, BottomRows: bottom}
}

// cropVertical returns a copy of img with `top` rows removed from the top
// and `bottom` rows removed from the bottom.
func cropVertical(img *image.RGBA, top, bottom int) *image.RGBA {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	newTop := top
	newBottom := h - bottom
	if newBottom < newTop {
		newBottom = newTop
	}
	out := image.NewRGBA(image.Rect(0, 0, w, newBottom-newTop))
	for y := newTop; y < newBottom; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y-newTop, img.At(x, y))
		}
	}
	return out
}

// Stitch glues a sequence of frames into one tall image, using
// FindOverlap between each consecutive pair. All frames must share the
// same width — Stitch returns *ErrDimensionMismatch immediately if not,
// rather than silently cropping to a common width and comparing
// misaligned content.
//
// Before matching, DetectStaticEdges checks for fixed UI (sticky nav,
// status bar) present unchanged across every frame and crops it from
// every frame uniformly — both so it can't produce a false full-height
// match (fixed UI is trivially "identical" at any offset, which is
// exactly the wrong kind of match) and so it doesn't get pasted into the
// output repeatedly, once per frame, as it otherwise would.
func Stitch(frames []*image.RGBA) (*image.RGBA, []FrameResult, StaticEdges, error) {
	if len(frames) == 0 {
		return nil, nil, StaticEdges{}, nil
	}

	expectedWidth := frames[0].Bounds().Dx()
	for i, f := range frames {
		if f.Bounds().Dx() != expectedWidth {
			return nil, nil, StaticEdges{}, &ErrDimensionMismatch{
				FrameIndex:    i,
				ExpectedWidth: expectedWidth,
				ActualWidth:   f.Bounds().Dx(),
			}
		}
	}

	static := DetectStaticEdges(frames)
	trimmed := make([]*image.RGBA, len(frames))
	for i, f := range frames {
		trimmed[i] = cropVertical(f, static.TopRows, static.BottomRows)
	}

	results := make([]FrameResult, 0, len(trimmed))
	current := trimmed[0]
	results = append(results, FrameResult{Index: 0, AddedPx: current.Bounds().Dy(), MatchScore: 1})

	for i := 1; i < len(trimmed); i++ {
		next := trimmed[i]

		overlap, score := FindOverlap(current, next)
		newRows := next.Bounds().Dy() - overlap

		combined := image.NewRGBA(image.Rect(0, 0, expectedWidth, current.Bounds().Dy()+newRows))
		for y := 0; y < current.Bounds().Dy(); y++ {
			for x := 0; x < expectedWidth; x++ {
				combined.Set(x, y, current.At(x, y))
			}
		}
		for y := 0; y < newRows; y++ {
			for x := 0; x < expectedWidth; x++ {
				combined.Set(x, current.Bounds().Dy()+y, next.At(x, overlap+y))
			}
		}
		current = combined

		results = append(results, FrameResult{
			Index:        i,
			OverlapPx:    overlap,
			AddedPx:      newRows,
			MatchScore:   score,
			LowConfident: overlap == 0,
		})
	}

	return current, results, static, nil
}

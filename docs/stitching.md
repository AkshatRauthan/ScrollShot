# How stitching works

This is the part of the codebase that took the most iteration, because it kept getting tested against real, reported bugs rather than synthetic assumptions. Every constant in `internal/stitch/stitch.go` has a specific false positive or false negative behind why it's set where it is — this document explains what those were, not just what the final numbers are.

## The pipeline, end to end

1. **Dimension validation** — reject immediately if frames don't share a width.
2. **Static-UI detection** — scan all frames at once for rows that never change (sticky navs, status bars); crop them out before anything else happens.
3. **Overlap search** (per consecutive frame pair):
   a. Cheap sparse scan across every possible offset.
   b. Dense, full-column re-verification on the best candidate and its best *distant* rival.
   c. Confidence check, margin check (or high-confidence override), background exclusion — all applied to the dense scores.
4. **Stitch** — trim the confirmed overlap, append the new content.

Each stage exists because skipping it produced a specific, real failure.

## Stage 0: dimension validation

**Why it exists:** a capture-backend switch, window resize, or DPI/monitor change mid-session used to get silently cropped to a common width before comparison — meaning two frames at completely different scales would still get compared, pixel (0,0) of one against pixel (0,0) of the other, producing corrupted output with zero indication anything had gone wrong.

**What it does now:** `Stitch()` checks every frame's width against the first frame's before doing anything else, and returns a hard, descriptive error — naming the mismatched frame and its actual width — if they don't match. Frames are left on disk for inspection rather than being consumed.

Only width is checked, not height — frames legitimately have different heights (the last capture in a session is often a shorter partial scroll), so requiring equal height would break normal usage.

## Stage 1: static-UI detection

**Why it exists:** a sticky nav bar or a fixed status bar is present, unchanged, in every single frame — which means it's *technically* a perfect match at any offset. Without special handling, this either gets duplicated once per frame in the output, or worse, gets mistaken for the actual scroll overlap (a status bar's presence at the bottom of every frame can produce a "100% match" between two frames that scrolled a full page, since the matcher's reference strip lands entirely on the status bar).

**How it works:** `DetectStaticEdges` scans from the top and from the bottom, independently, across *all* frames in the session at once — not just consecutive pairs, since two frames could coincidentally agree by chance, but something unchanged across every frame in the whole session is a much stronger signal it's genuinely fixed.

For each candidate row, it checks: does this row (at the same absolute position) match, within tolerance, across every frame?

**The background-domination trap, twice.** The first version of this check had no exclusion logic and was never tested against content where a uniform background dominates (a dark code editor). On that content, background-vs-background trivially "matched" across frames regardless of position, causing large false-positive crops. The first fix — exclude near-background pixels from judgment — went too far the other way: a genuinely solid-colored nav bar's own color usually *is* the frame's dominant color (it's a big uniform block), so excluding "background-colored" pixels excluded the nav bar's own judgment entirely, and it stopped being detected at all.

The actual fix: judge on non-background columns when there are enough of them to judge on meaningfully; fall back to judging the whole row only when it's genuinely uniform (too few distinctive columns to assess separately). This correctly handles both a sparse-text-on-dark-background row (judge only the sparse text) and a solid-color nav row (nothing to exclude from, so judge the whole thing).

**Why full-width scanning, not sparse sampling.** An early version sampled ~150 evenly-spaced columns (matching the overlap search's approach) for speed. But static-edge detection only ever examines a small number of candidate rows once each — unlike overlap search, which repeats for every possible offset — so it can afford to scan every column. Sparse sampling was missing sparse real content (scattered syntax-highlighted specks) in certain rows purely by bad luck of which columns got sampled, wrongly classifying content-bearing rows as uniform.

**Why the cap is 12%, not higher.** `maxStaticFraction` limits how much of a frame's edge can ever be classified as static, as a safety net. It was originally 30%, tightened to 12% based on the observation that real fixed UI (nav bars, status bars) rarely exceeds ~10% of a window's height in practice — a looser cap mainly gave more room for degenerate cases (see [Known Limitations](known-limitations.md)) to over-crop.

**Tolerance is intentionally stricter here than in overlap matching.** `staticEdgeTolerance` (60) is separate from `pointTolerance` (90, used for overlap scoring) on purpose. A false positive in static detection destructively crops real content before matching ever runs on it — there's no recovery. A false positive in overlap matching just means a rejected match and a retry. The two constants were originally shared, and loosening one for a real matching fix silently loosened the other, causing new static-detection false positives — that's why they're now decoupled.

## Stage 2: the overlap search itself

**Two-pass, not one-pass.** The original design scored every candidate offset once, sparsely, and picked the best. On low-entropy repetitive content (a dark editor with sparse text), that single-pass sparse score could lock onto a coincidentally-similar wrong offset. The fix: a cheap sparse scan across every offset first (for speed — checking every offset densely would be far too slow), which shortlists only the best candidate and its best *distant* rival (excluding nearby offsets, which naturally score similarly to a true match since the content barely shifts between them). Those two shortlisted candidates then get a dense, full-column re-verification over a taller window before any accept/reject decision is made.

**Background exclusion, again — same trap as Stage 1.** The overlap-matching score also excludes near-background pixels for the same reason: on content where 98%+ of pixels are a uniform background, background-vs-background agreement dominates the score regardless of whether the real, distinctive content actually lines up. Excluding it keeps the score meaningful.

## Stage 3: accept/reject decision

Three checks, applied to the dense scores:

**Confidence bar** (`minMatchScore = 0.90`) — the winning candidate has to be a good match on its own, full stop.

**Margin bar** (`minMargin = 0.06`) — the winner has to clearly outscore its best distant rival, not just barely edge it out. This exists because repetitive content (many code blocks, grid layouts) can produce multiple candidates that all score reasonably well; requiring a clear winner catches genuine ambiguity that a bare confidence threshold would miss.

**High-confidence override** (`highConfidenceOverride = 0.985`) — a candidate scoring at or above this bypasses the margin check entirely. This was added after real logs showed genuine, correct matches scoring 99–100% still getting rejected by the margin bar — on structurally repetitive source code, a distant, *wrong* rival can also score deceptively high, and a match this close to perfect shouldn't need to also out-score that rival to be trusted.

The override's threshold was set from measurement, not intuition: a deliberately adversarial test (repetitive code-like content combined with realistic anti-aliasing jitter) found a genuinely *wrong* offset scoring as high as 97.1%. The true match on the same content scored 99.67%. The threshold sits at 0.985 — comfortably above the measured false-positive ceiling, comfortably below the measured true-positive floor, with margin on both sides rather than sitting exactly between them.

## The tolerance-for-noise problem

Two independently-captured screenshots of the *same* logical content are never byte-identical — font anti-aliasing and sub-pixel rendering vary slightly between renders, even with zero scroll. The original `pointTolerance` (60) was calibrated against clean synthetic test data with no such noise, and was too strict for this: real matches were scoring 83–88% and getting rejected outright.

The fix was calibrated against reality, not guessed: a synthetic jitter test was built and swept across tolerance values until it reproduced the actual observed real-world scores (83–88%), confirming the test was representative. Then the tolerance was swept again to find where the score recovered to near-100% — landing on `pointTolerance = 90`.

## What "score" actually means

Match scoring is a **percentage of sampled points that agree within a per-pixel tolerance**, not a raw mean pixel difference. This matters: a mean is fragile — a handful of genuinely different pixels (a blinking cursor, a live timestamp, a hover state) can swing a mean noticeably even when the vast majority of a region matches perfectly. A tolerant match-percentage absorbs that kind of small, localized noise without needing to special-case what caused it.

## Further reading

[`docs/known-limitations.md`](known-limitations.md) covers the cases this pipeline still doesn't fully resolve — worth reading before assuming a given edge case is a bug rather than a documented, understood gap.
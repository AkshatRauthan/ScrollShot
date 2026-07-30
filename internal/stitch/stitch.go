// Package stitch contains the scrolling-screenshot stitching engine:
// given a sequence of overlapping frame images, it finds where each
// consecutive pair overlaps and glues them into one tall image.
//
// This package is deliberately independent of how frames were captured —
// it just takes image.Image in and produces image.RGBA out — so it works
// unchanged regardless of which capture backend produced the frames.
package stitch

import "image"

// FrameResult reports what happened when stitching one frame onto the
// growing image, for logging/debugging (e.g. spotting a 0px-overlap
// frame that might indicate a missed match rather than a real jump).
type FrameResult struct {
	Index        int
	OverlapPx    int
	AddedPx      int
	LowConfident bool // true if overlap was 0 — could be a real scroll jump
	// or a failed match; caller should decide how to warn/handle.
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

// stripHeight is the thickness of the reference strip taken from the
// bottom of the previous frame when searching for the overlap point.
const stripHeight = 20

// FindOverlap locates how many rows at the bottom of `top` are duplicated
// at the top of `bottom` (the region scrolled past but re-captured). It
// slides a thin reference strip from the bottom of `top` down through
// `bottom`, picking the position with the lowest pixel difference.
// Sampling a subset of columns keeps this fast on full-resolution
// screenshots. Returns 0 if no confident match is found.
func FindOverlap(top, bottom *image.RGBA) int {
	hTop := top.Bounds().Dy()
	hBot := bottom.Bounds().Dy()
	w := top.Bounds().Dx()

	if hTop < stripHeight || hBot < stripHeight {
		return 0
	}

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
		return 0
	}

	bestOffset := -1
	bestScore := -1.0

	for offset := 0; offset <= maxOffset; offset++ {
		var total float64
		var count int
		for _, x := range cols {
			for k := 0; k < stripHeight; k++ {
				tr, tg, tb, _ := top.At(x, hTop-stripHeight+k).RGBA()
				br, bg, bb, _ := bottom.At(x, offset+k).RGBA()
				total += absDiff(tr, br) + absDiff(tg, bg) + absDiff(tb, bb)
				count++
			}
		}
		if count == 0 {
			continue
		}
		score := total / float64(count)
		if bestScore < 0 || score < bestScore {
			bestScore = score
			bestOffset = offset
		}
		if score < 50 { // near-perfect match, stop early
			break
		}
	}

	// no decent match found — frames likely don't overlap at all
	if bestOffset < 0 || bestScore > 1500 {
		return 0
	}
	return stripHeight + bestOffset
}

func absDiff(a, b uint32) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}

// Stitch glues a sequence of frames into one tall image, using FindOverlap
// between each consecutive pair. Returns the final image plus a per-frame
// report of how each join went (for logging/warnings by the caller).
func Stitch(frames []*image.RGBA) (*image.RGBA, []FrameResult) {
	if len(frames) == 0 {
		return nil, nil
	}

	results := make([]FrameResult, 0, len(frames))
	current := frames[0]
	results = append(results, FrameResult{Index: 0, AddedPx: current.Bounds().Dy()})

	for i := 1; i < len(frames); i++ {
		next := frames[i]

		w := current.Bounds().Dx()
		if next.Bounds().Dx() < w {
			w = next.Bounds().Dx()
		}

		overlap := FindOverlap(current, next)
		newRows := next.Bounds().Dy() - overlap

		combined := image.NewRGBA(image.Rect(0, 0, w, current.Bounds().Dy()+newRows))
		for y := 0; y < current.Bounds().Dy(); y++ {
			for x := 0; x < w; x++ {
				combined.Set(x, y, current.At(x, y))
			}
		}
		for y := 0; y < newRows; y++ {
			for x := 0; x < w; x++ {
				combined.Set(x, current.Bounds().Dy()+y, next.At(x, overlap+y))
			}
		}
		current = combined

		results = append(results, FrameResult{
			Index:        i,
			OverlapPx:    overlap,
			AddedPx:      newRows,
			LowConfident: overlap == 0,
		})
	}

	return current, results
}

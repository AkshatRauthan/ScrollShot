// Package testutil provides synthetic test-image generators shared
// across the project's test suites. It exists so that new tests — for
// features not yet built (edit, export, autoscroll) as much as for
// stitch and session today — can generate realistic-enough frames
// without each test file reinventing image generation from scratch.
//
// This is a regular (non-_test.go) package specifically so it's
// importable from any package's test files, following the same
// "shared infrastructure, pluggable specifics" spirit as
// internal/capture's backend registry: add a new generator here once,
// and every test package can use it.
//
// Nothing in this package should be imported by non-test code — it
// exists purely to make tests easier to write and read.
package testutil

import (
	"image"
	"image/color"
	"math/rand"
)

// RandomPage generates a frame where each row is a distinct, uniformly
// random color across its full width — high-entropy content with no
// repeated structure. Useful as a baseline "well-behaved" test case: if
// a match/rejection doesn't work on this, the problem isn't ambiguity,
// it's a real bug.
func RandomPage(seed int64, width, height int) *image.RGBA {
	r := rand.New(rand.NewSource(seed))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		c := color.RGBA{uint8(r.Intn(255)), uint8(r.Intn(255)), uint8(r.Intn(255)), 255}
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// CodeEditorPage generates a frame simulating a dark-theme code editor:
// a dominant uniform dark background with sparse, syntax-colored
// "text" specks arranged in rows. This is deliberately low-entropy and
// background-dominated (typically 95%+ background pixels) — it's the
// content shape that originally exposed the background-domination bugs
// in both overlap matching and static-edge detection, so it's the right
// shape to regression-test against, not just a cosmetic choice.
func CodeEditorPage(seed int64, width, height int) *image.RGBA {
	r := rand.New(rand.NewSource(seed))
	bg := color.RGBA{30, 30, 35, 255}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bg)
		}
	}
	palette := []color.RGBA{
		{86, 156, 214, 255}, {206, 145, 120, 255}, {181, 206, 168, 255}, {220, 220, 170, 255},
	}
	lines := 40
	lineHeight := height / lines
	if lineHeight < 1 {
		lineHeight = 1
	}
	for line := 0; line < lines; line++ {
		lineY := line * lineHeight
		numSpecks := 15 + r.Intn(10)
		for s := 0; s < numSpecks; s++ {
			x := r.Intn(width - 4)
			speckW := 2 + r.Intn(6)
			c := palette[r.Intn(len(palette))]
			for dy := 2; dy < lineHeight-2 && dy < 12; dy++ {
				for dx := 0; dx < speckW; dx++ {
					if x+dx < width && lineY+dy < height {
						img.Set(x+dx, lineY+dy, c)
					}
				}
			}
		}
	}
	return img
}

// RenderedTextPage generates a frame simulating rendered prose/code
// text: per-line variable-length runs of "characters" at randomized
// positions, non-repetitive across lines (unlike CodeEditorPage, whose
// 40-line block structure repeats). Use this specifically when a test
// needs realistic text-like content WITHOUT structural repetition —
// e.g. isolating anti-aliasing-noise tolerance from repetition-driven
// ambiguity, which is why the two generators are kept separate rather
// than merged into one "text-like" generator.
func RenderedTextPage(seed int64, width, height int) *image.RGBA {
	r := rand.New(rand.NewSource(seed))
	bg := color.RGBA{30, 30, 35, 255}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bg)
		}
	}
	lineHeight := 18
	textColor := color.RGBA{210, 210, 215, 255}
	for line := 0; line*lineHeight < height; line++ {
		y := line * lineHeight
		lineLen := 20 + r.Intn(60)
		x := 5
		for c := 0; c < lineLen && x < width-3; c++ {
			charW := 5 + r.Intn(4)
			if r.Intn(4) != 0 {
				for dy := 3; dy < 12; dy++ {
					for dx := 0; dx < charW-1; dx++ {
						if x+dx < width && y+dy < height {
							img.Set(x+dx, y+dy, textColor)
						}
					}
				}
			}
			x += charW
		}
	}
	return img
}

// RepeatingBlockPage generates a frame with genuinely repeating
// structural blocks (like repeated code patterns — a function shape
// repeated with per-instance variation) every blockHeight pixels. This
// is deliberately harder than CodeEditorPage's coarse repetition: it's
// the shape of content that originally produced a distant, genuinely
// high-scoring false-positive rival during real-world calibration —
// keep using this generator (not a milder one) for any future
// margin/override-threshold regression test.
func RepeatingBlockPage(seed int64, width, height, blockHeight int) *image.RGBA {
	r := rand.New(rand.NewSource(seed))
	bg := color.RGBA{30, 30, 35, 255}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bg)
		}
	}
	palette := []color.RGBA{{86, 156, 214, 255}, {206, 145, 120, 255}, {181, 206, 168, 255}}
	for blockY := 0; blockY < height; blockY += blockHeight {
		for line := 0; line < 5; line++ {
			ly := blockY + line*16
			if ly+10 >= height {
				break
			}
			numSpecks := 10 + r.Intn(5)
			for s := 0; s < numSpecks; s++ {
				x := r.Intn(width - 4)
				speckW := 3 + r.Intn(30)
				c := palette[r.Intn(len(palette))]
				for dy := 2; dy < 10; dy++ {
					for dx := 0; dx < speckW; dx++ {
						if x+dx < width && ly+dy < height {
							img.Set(x+dx, ly+dy, c)
						}
					}
				}
			}
		}
	}
	return img
}

// Crop returns the sub-image of full spanning rows [top, bottom).
func Crop(full *image.RGBA, top, bottom int) *image.RGBA {
	w := full.Bounds().Dx()
	out := image.NewRGBA(image.Rect(0, 0, w, bottom-top))
	for y := top; y < bottom; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y-top, full.At(x, y))
		}
	}
	return out
}

// Paste copies src into dst starting at row atY.
func Paste(dst, src *image.RGBA, atY int) {
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			dst.Set(x, atY+y, src.At(x, y))
		}
	}
}

// SolidBlock generates a uniform-color frame of the given size — for
// simulating fixed UI elements like a nav bar or status bar.
func SolidBlock(width, height int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// Jitter simulates two independent renders of logically-identical
// content: applies a small random per-pixel, per-channel delta,
// bounded to +/-amount. This is what makes a test representative of
// real screenshot noise (anti-aliasing, font hinting) rather than the
// unrealistic byte-for-byte-identical crops a naive test would use —
// real matches need to tolerate this; false matches should still be
// caught despite it.
func Jitter(img *image.RGBA, seed int64, amount int) *image.RGBA {
	r := rand.New(rand.NewSource(seed))
	b := img.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rr, gg, bb, aa := img.At(x, y).RGBA()
			delta := func(v uint32) uint8 {
				d := r.Intn(amount*2+1) - amount
				nv := int(v>>8) + d
				if nv < 0 {
					nv = 0
				}
				if nv > 255 {
					nv = 255
				}
				return uint8(nv)
			}
			out.Set(x, y, color.RGBA{delta(rr), delta(gg), delta(bb), uint8(aa >> 8)})
		}
	}
	return out
}

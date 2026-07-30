// Package export handles writing the final stitched image to disk with
// size control: lossless recompression (tighter PNG compression, same
// pixels) or lossy scale-down/re-encode for when file size matters more
// than pixel-perfect fidelity.
//
// Not yet implemented — this file locks in the contract so main.go can
// be wired up to call it later (e.g. via --lossless / --compress flags)
// without another restructure.
package export

import (
	"errors"
	"image"
)

// ErrNotImplemented is returned by every function in this package until
// they're built out.
var ErrNotImplemented = errors.New("export: not yet implemented")

// Mode selects how Save should trade off file size against fidelity.
type Mode int

const (
	// Lossless keeps every pixel exact, just compresses harder than the
	// default PNG settings (e.g. png.BestCompression).
	Lossless Mode = iota
	// Lossy re-encodes at reduced quality/resolution for a much smaller
	// file, for cases where exact pixels don't matter (quick sharing).
	Lossy
)

// Options controls how Save writes the final image.
type Options struct {
	Mode Mode
	// MaxWidth, if nonzero, downscales the image (preserving aspect
	// ratio) before saving. Applies in both modes.
	MaxWidth int
	// Quality applies only in Lossy mode (e.g. WebP/JPEG quality 1-100).
	Quality int
}

// Save writes img to path according to opts.
func Save(img image.Image, path string, opts Options) error {
	return ErrNotImplemented
}

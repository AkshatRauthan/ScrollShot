// Package edit provides pre-stitch adjustments to captured frames:
// cropping off a fixed region (e.g. a sticky header/nav bar that gets
// re-captured in every frame) and reordering frames if a session was
// captured out of sequence.
//
// Not yet implemented — this file locks in the contract so session/ and
// stitch/ can be wired up to call it later without another restructure.
package edit

import (
	"errors"
	"image"
)

// ErrNotImplemented is returned by every function in this package until
// they're built out.
var ErrNotImplemented = errors.New("edit: not yet implemented")

// CropTop removes the top `pixels` rows from img — for trimming a
// sticky header/nav bar that gets recaptured identically in every frame
// and would otherwise just add noise to the overlap search.
func CropTop(img image.Image, pixels int) (image.Image, error) {
	return nil, ErrNotImplemented
}

// CropBottom removes the bottom `pixels` rows from img — same idea as
// CropTop, for footers or fixed bottom bars.
func CropBottom(img image.Image, pixels int) (image.Image, error) {
	return nil, ErrNotImplemented
}

// Reorder takes the session's frame paths (in original capture order)
// and a desired new order (indices into framePaths), and returns the
// frame paths in that new order — for fixing a session that was
// captured out of sequence before it gets handed to stitch.Stitch.
func Reorder(framePaths []string, newOrder []int) ([]string, error) {
	return nil, ErrNotImplemented
}

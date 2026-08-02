package edit

import (
	"errors"
	"image"
	"testing"
)

// These tests lock in the CURRENT contract: every function in this
// package returns ErrNotImplemented. That's deliberate — the function
// signatures were decided ahead of implementation (see edit.go's doc
// comment) so callers elsewhere in the codebase can be wired up without
// a future restructure. When one of these gets implemented for real,
// its test here MUST be rewritten to test real behavior — it can't just
// keep passing, because a real implementation will stop returning
// ErrNotImplemented. That forced failure is the point: it stops anyone
// from accidentally leaving a "not implemented" test in place after
// implementing the function it covers.

func TestCropTop_NotYetImplemented(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	_, err := CropTop(img, 5)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("CropTop is now implemented (returned %v instead of ErrNotImplemented) — "+
			"replace this test with real behavioral tests, don't just update the expected error", err)
	}
}

func TestCropBottom_NotYetImplemented(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	_, err := CropBottom(img, 5)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("CropBottom is now implemented (returned %v instead of ErrNotImplemented) — "+
			"replace this test with real behavioral tests, don't just update the expected error", err)
	}
}

func TestReorder_NotYetImplemented(t *testing.T) {
	_, err := Reorder([]string{"a.png", "b.png"}, []int{1, 0})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Reorder is now implemented (returned %v instead of ErrNotImplemented) — "+
			"replace this test with real behavioral tests, don't just update the expected error", err)
	}
}

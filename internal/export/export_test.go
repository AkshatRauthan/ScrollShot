package export

import (
	"errors"
	"image"
	"testing"
)

// See edit/edit_test.go's doc comment for why these tests assert
// ErrNotImplemented rather than being skipped or omitted — the same
// reasoning applies here.

func TestSave_Lossless_NotYetImplemented(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	err := Save(img, "/tmp/does-not-matter.png", Options{Mode: Lossless})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Save (Lossless) is now implemented (returned %v instead of ErrNotImplemented) — "+
			"replace this test with real behavioral tests (verify actual file output, compression), "+
			"don't just update the expected error", err)
	}
}

func TestSave_Lossy_NotYetImplemented(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	err := Save(img, "/tmp/does-not-matter.png", Options{Mode: Lossy, Quality: 80, MaxWidth: 800})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Save (Lossy) is now implemented (returned %v instead of ErrNotImplemented) — "+
			"replace this test with real behavioral tests, don't just update the expected error", err)
	}
}

func TestModeConstants_AreDistinct(t *testing.T) {
	// Locks in that Lossless and Lossy are, and remain, different values
	// — a trivial-looking check, but it's the kind of thing an
	// accidental refactor (e.g. reordering the const block) could break
	// silently since both are just small integers.
	if Lossless == Lossy {
		t.Fatalf("Lossless and Lossy must be distinct Mode values, both equal %v", Lossless)
	}
}

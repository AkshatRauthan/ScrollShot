package stitch

import (
	"testing"

	"scrollshot/internal/testutil"
)

func TestFindOverlap_Direct(t *testing.T) {
	// Directly test the FindOverlap function which was recently modified to return OverlapResult.
	full := testutil.RandomPage(99, 400, 1000)

	f1 := ToRGBA(testutil.Crop(full, 0, 500))
	f2 := ToRGBA(testutil.Crop(full, 350, 850)) // 150px overlap

	res := FindOverlap(f1, f2)

	if res.OverlapPx != 150 {
		t.Errorf("expected overlap of 150px, got %dpx", res.OverlapPx)
	}
	if res.MatchScore < 0.99 {
		t.Errorf("expected high match score, got %f", res.MatchScore)
	}
}

func TestFindOverlap_NoOverlap(t *testing.T) {
	f1 := ToRGBA(testutil.RandomPage(101, 400, 500))
	f2 := ToRGBA(testutil.RandomPage(102, 400, 500))

	res := FindOverlap(f1, f2)

	if res.OverlapPx != 0 {
		t.Errorf("expected 0 overlap for unrelated frames, got %dpx", res.OverlapPx)
	}
}

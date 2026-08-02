package session

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestSession creates a Session pointed at a fresh temp directory,
// isolated per test — avoids relying on $HOME or touching the real
// ~/.cache/scrollshot_session, and gives each test a clean slate
// without needing manual cleanup (t.TempDir handles that).
func newTestSession(t *testing.T) *Session {
	t.Helper()
	return &Session{dir: t.TempDir()}
}

func solidImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestSession_SaveFrame_SequentialNaming(t *testing.T) {
	s := newTestSession(t)
	img := solidImage(10, 10, color.RGBA{255, 0, 0, 255})

	for i := 0; i < 3; i++ {
		idx, path, err := s.SaveFrame(img)
		if err != nil {
			t.Fatalf("SaveFrame #%d: unexpected error: %v", i, err)
		}
		if idx != i {
			t.Fatalf("SaveFrame #%d: expected index %d, got %d", i, i, idx)
		}
		wantPath := filepath.Join(s.dir, "frame_000"+string(rune('0'+i))+".png")
		if path != wantPath {
			t.Fatalf("SaveFrame #%d: expected path %s, got %s", i, wantPath, path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("SaveFrame #%d: file not actually created at %s: %v", i, path, err)
		}
	}
}

func TestSession_Frames_ReturnsSortedOrder(t *testing.T) {
	s := newTestSession(t)
	img := solidImage(5, 5, color.RGBA{0, 255, 0, 255})

	const n = 12 // deliberately > 10, to catch lexicographic-vs-numeric sort bugs
	for i := 0; i < n; i++ {
		if _, _, err := s.SaveFrame(img); err != nil {
			t.Fatalf("SaveFrame #%d: %v", i, err)
		}
	}

	frames, err := s.Frames()
	if err != nil {
		t.Fatalf("Frames(): unexpected error: %v", err)
	}
	if len(frames) != n {
		t.Fatalf("expected %d frames, got %d: %v", n, len(frames), frames)
	}
	for i, f := range frames {
		want := filepath.Join(s.dir, "frame_"+padZero(i)+".png")
		if f != want {
			t.Fatalf("frame at position %d: expected %s, got %s (full list: %v) — "+
				"likely a zero-padding/lexicographic sort issue once index reaches double digits",
				i, want, f, frames)
		}
	}
}

func padZero(i int) string {
	s := "0000"
	digits := []rune{}
	n := i
	if n == 0 {
		digits = []rune{'0'}
	}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	return s[:4-len(digits)] + string(digits)
}

func TestSession_Frames_EmptySessionReturnsEmptyNotError(t *testing.T) {
	s := newTestSession(t)
	frames, err := s.Frames()
	if err != nil {
		t.Fatalf("unexpected error on empty session: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("expected 0 frames in a fresh session, got %d: %v", len(frames), frames)
	}
}

func TestSession_EnsureFresh_CreatesDirIfMissing(t *testing.T) {
	s := &Session{dir: filepath.Join(t.TempDir(), "does-not-exist-yet")}
	if _, err := os.Stat(s.dir); !os.IsNotExist(err) {
		t.Fatalf("test setup error: directory should not exist yet")
	}

	if _, err := s.EnsureFresh(); err != nil {
		t.Fatalf("EnsureFresh: unexpected error: %v", err)
	}
	if _, err := os.Stat(s.dir); err != nil {
		t.Fatalf("expected EnsureFresh to create the session directory, but it still doesn't exist: %v", err)
	}
}

func TestSession_EnsureFresh_KeepsRecentSession(t *testing.T) {
	// A session with a recently-modified frame must NOT be cleared —
	// this is the normal case (mid-capture), and clearing it would lose
	// the user's in-progress work.
	s := newTestSession(t)
	img := solidImage(5, 5, color.RGBA{0, 0, 255, 255})
	if _, _, err := s.SaveFrame(img); err != nil {
		t.Fatalf("SaveFrame: %v", err)
	}

	cleared, err := s.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: unexpected error: %v", err)
	}
	if cleared {
		t.Fatalf("expected a freshly-saved frame's session to be kept, but EnsureFresh reported it cleared as stale")
	}
	frames, _ := s.Frames()
	if len(frames) != 1 {
		t.Fatalf("expected the recent frame to survive EnsureFresh, but found %d frames", len(frames))
	}
}

func TestSession_EnsureFresh_ClearsStaleSession(t *testing.T) {
	// A session whose last frame is older than StaleGap must be treated
	// as an abandoned, never-finished session and cleared automatically
	// — this is what lets `capture` skip a separate `start` step.
	s := newTestSession(t)
	img := solidImage(5, 5, color.RGBA{255, 255, 0, 255})
	_, path, err := s.SaveFrame(img)
	if err != nil {
		t.Fatalf("SaveFrame: %v", err)
	}

	// Backdate the frame's mtime past StaleGap without actually waiting.
	staleTime := time.Now().Add(-StaleGap - time.Minute)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatalf("test setup: could not backdate frame mtime: %v", err)
	}

	cleared, err := s.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: unexpected error: %v", err)
	}
	if !cleared {
		t.Fatalf("expected a session whose last frame is older than StaleGap (%v) to be cleared, but it wasn't", StaleGap)
	}
	frames, _ := s.Frames()
	if len(frames) != 0 {
		t.Fatalf("expected stale frames to be removed, but %d remain: %v", len(frames), frames)
	}
}

func TestSession_EnsureFresh_BoundaryJustUnderStaleGap_NotCleared(t *testing.T) {
	// Boundary case: a frame just barely within StaleGap must survive —
	// guards against an off-by-one in the staleness comparison.
	s := newTestSession(t)
	img := solidImage(5, 5, color.RGBA{100, 100, 100, 255})
	_, path, err := s.SaveFrame(img)
	if err != nil {
		t.Fatalf("SaveFrame: %v", err)
	}

	justUnder := time.Now().Add(-StaleGap + 10*time.Second)
	if err := os.Chtimes(path, justUnder, justUnder); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	cleared, err := s.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: unexpected error: %v", err)
	}
	if cleared {
		t.Fatalf("expected a frame just under StaleGap old to survive, but it was cleared")
	}
}

func TestSession_Clear_RemovesAllFrames(t *testing.T) {
	s := newTestSession(t)
	img := solidImage(5, 5, color.RGBA{50, 50, 50, 255})
	for i := 0; i < 5; i++ {
		if _, _, err := s.SaveFrame(img); err != nil {
			t.Fatalf("SaveFrame #%d: %v", i, err)
		}
	}

	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: unexpected error: %v", err)
	}
	frames, err := s.Frames()
	if err != nil {
		t.Fatalf("Frames() after Clear: unexpected error: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("expected 0 frames after Clear, got %d: %v", len(frames), frames)
	}
}

func TestSession_Clear_EmptySessionDoesNotError(t *testing.T) {
	s := newTestSession(t)
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear on an already-empty session should not error, got: %v", err)
	}
}

func TestSession_Dir_ReturnsConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	s := &Session{dir: dir}
	if s.Dir() != dir {
		t.Fatalf("expected Dir() to return %s, got %s", dir, s.Dir())
	}
}

func TestSession_New_ReturnsPathUnderHomeCache(t *testing.T) {
	// Sanity check on the real constructor (not the test helper) — this
	// is the one path in the package that actually depends on the real
	// environment, so it's tested separately and lightly.
	s, err := New()
	if err != nil {
		t.Fatalf("New(): unexpected error: %v", err)
	}
	if s.Dir() == "" {
		t.Fatalf("New() produced an empty directory path")
	}
	home, _ := os.UserHomeDir()
	wantSuffix := filepath.Join(".cache", "scrollshot_session")
	if filepath.Join(home, wantSuffix) != s.Dir() {
		t.Fatalf("expected New() to return %s, got %s", filepath.Join(home, wantSuffix), s.Dir())
	}
}

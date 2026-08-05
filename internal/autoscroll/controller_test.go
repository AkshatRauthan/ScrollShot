package autoscroll

import (
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"scrollshot/internal/session"
)

// mockCapturer provides controlled fake frames for testing.
type mockCapturer struct {
	frames     []image.Image
	captureErr error
	callCount  int
}

func (m *mockCapturer) Name() string    { return "mock" }
func (m *mockCapturer) Available() bool { return true }
func (m *mockCapturer) CaptureActiveWindow() (image.Image, error) {
	if m.captureErr != nil {
		return nil, m.captureErr
	}
	if m.callCount >= len(m.frames) {
		return nil, errors.New("out of mock frames")
	}
	img := m.frames[m.callCount]
	m.callCount++
	return img, nil
}

// mockScroller records scroll calls and returns a controlled error.
type mockScroller struct {
	scrollErr error
	scrolls   []int
}

func (m *mockScroller) Name() string    { return "mock" }
func (m *mockScroller) Available() bool { return true }
func (m *mockScroller) ScrollDown(amountPx int) error {
	m.scrolls = append(m.scrolls, amountPx)
	return m.scrollErr
}
func (m *mockScroller) Close() error { return nil }

func setupTestSession(t *testing.T) *session.Session {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("LOCALAPPDATA", tmp)
	t.Setenv("HOME", tmp)

	s, err := session.New()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if _, err := s.EnsureFresh(); err != nil {
		t.Fatalf("failed to ensure fresh session: %v", err)
	}
	return s
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

func TestController_Run_Success(t *testing.T) {
	// A simple successful run that stops because it hit the bottom (simulated
	// by identical consecutive frames, triggering the stagnation check).
	sess := setupTestSession(t)

	cfg := DefaultConfig()
	cfg.MaxFrames = 5
	cfg.Delay = 1 * time.Millisecond // fast for tests
	cfg.StagnationLimit = 2

	mc := &mockCapturer{
		frames: []image.Image{
			solidImage(100, 100, color.RGBA{255, 0, 0, 255}), // frame 0
			solidImage(100, 100, color.RGBA{0, 255, 0, 255}), // frame 1
			solidImage(100, 100, color.RGBA{0, 0, 255, 255}), // frame 2
			// Frame 3 and 4 are identical to 2, simulating "hit the bottom"
			solidImage(100, 100, color.RGBA{0, 0, 255, 255}), // frame 3
			solidImage(100, 100, color.RGBA{0, 0, 255, 255}), // frame 4
		},
	}
	ms := &mockScroller{}

	ctrl, err := New(mc, ms, sess, cfg)
	if err != nil {
		t.Fatalf("unexpected error creating controller: %v", err)
	}

	result, err := ctrl.Run()
	if err != nil {
		t.Fatalf("unexpected error running controller: %v", err)
	}

	if result.FramesCaptured != 5 {
		t.Errorf("expected 5 frames captured, got %d", result.FramesCaptured)
	}
	if result.StopReason != StopStagnation {
		t.Errorf("expected StopReason=StopStagnation, got %v", result.StopReason)
	}
	if len(ms.scrolls) != 4 {
		t.Errorf("expected 4 scrolls, got %d", len(ms.scrolls))
	}
}

func TestController_Run_MaxFrames(t *testing.T) {
	// Reaches max frames before stagnating.
	sess := setupTestSession(t)

	cfg := DefaultConfig()
	cfg.MaxFrames = 3
	cfg.Delay = 1 * time.Millisecond

	mc := &mockCapturer{
		frames: []image.Image{
			solidImage(100, 100, color.RGBA{1, 0, 0, 255}),
			solidImage(100, 100, color.RGBA{2, 0, 0, 255}),
			solidImage(100, 100, color.RGBA{3, 0, 0, 255}),
			solidImage(100, 100, color.RGBA{4, 0, 0, 255}),
		},
	}
	ms := &mockScroller{}

	ctrl, err := New(mc, ms, sess, cfg)
	if err != nil {
		t.Fatalf("unexpected error creating controller: %v", err)
	}

	result, err := ctrl.Run()
	if err != nil {
		t.Fatalf("unexpected error running controller: %v", err)
	}

	if result.FramesCaptured != 3 {
		t.Errorf("expected exactly max frames (3), got %d", result.FramesCaptured)
	}
	if result.StopReason != StopMaxFrames {
		t.Errorf("expected StopReason=StopMaxFrames, got %v", result.StopReason)
	}
}

func TestController_New_Validation(t *testing.T) {
	sess := setupTestSession(t)
	cfg := DefaultConfig()
	mc := &mockCapturer{}
	ms := &mockScroller{}

	if _, err := New(nil, ms, sess, cfg); err == nil {
		t.Error("expected error for nil capturer")
	}
	if _, err := New(mc, nil, sess, cfg); err == nil {
		t.Error("expected error for nil scroller")
	}
	if _, err := New(mc, ms, nil, cfg); err == nil {
		t.Error("expected error for nil session")
	}
}

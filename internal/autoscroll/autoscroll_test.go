package autoscroll

import (
	"errors"
	"testing"
)

// mockScroller is a fake backend for testing the registry mechanics in
// isolation, mirroring internal/capture/capture_test.go's mockCapturer
// — this package's registry is deliberately built on the same pattern,
// so its tests follow the same shape.
type mockScroller struct {
	name      string
	available bool
	atBottom  bool
}

func (m *mockScroller) Name() string                  { return m.name }
func (m *mockScroller) Available() bool               { return m.available }
func (m *mockScroller) ScrollDown(amountPx int) error { return ErrNotImplemented }
func (m *mockScroller) AtBottom() (bool, error)       { return m.atBottom, nil }

// withIsolatedRegistry swaps the package-level registry for the
// duration of a test. Unlike internal/capture, no real backends are
// registered yet (no build-tag-gated files exist for this package), so
// isolation here is more about test hygiene and matching the sibling
// package's pattern than working around real registered backends today
// — but it protects these tests against breaking silently once a real
// backend IS added later.
func withIsolatedRegistry(t *testing.T) {
	t.Helper()
	original := registry
	registry = nil
	t.Cleanup(func() { registry = original })
}

func TestRegistry_Detect_ReturnsFirstAvailable(t *testing.T) {
	withIsolatedRegistry(t)
	Register(&mockScroller{name: "unavailable", available: false})
	Register(&mockScroller{name: "available", available: true})

	got := Detect()
	if got == nil {
		t.Fatalf("expected Detect() to return a backend, got nil")
	}
	if got.Name() != "available" {
		t.Fatalf("expected Detect() to return the first available backend, got %q", got.Name())
	}
}

func TestRegistry_Detect_ReturnsNilWhenNoneAvailable(t *testing.T) {
	withIsolatedRegistry(t)
	Register(&mockScroller{name: "unavailable", available: false})

	got := Detect()
	if got != nil {
		t.Fatalf("expected Detect() to return nil when nothing is available, got %q", got.Name())
	}
}

func TestRegistry_Detect_EmptyRegistryReturnsNil(t *testing.T) {
	withIsolatedRegistry(t)
	got := Detect()
	if got != nil {
		t.Fatalf("expected Detect() on an empty registry to return nil, got %q", got.Name())
	}
}

func TestMockScroller_ScrollDown_ReturnsNotImplemented(t *testing.T) {
	m := &mockScroller{name: "test"}
	err := m.ScrollDown(100)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected mockScroller.ScrollDown to return ErrNotImplemented (no real backend exists "+
			"yet to test against), got %v", err)
	}
}

func TestMockScroller_AtBottom_ReturnsConfiguredValue(t *testing.T) {
	m := &mockScroller{name: "test", atBottom: true}
	got, err := m.AtBottom()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Fatalf("expected AtBottom() to return true as configured, got false")
	}
}

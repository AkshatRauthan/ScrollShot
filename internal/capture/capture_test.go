package capture

import (
	"errors"
	"image"
	"testing"
)

// mockCapturer is a fake backend used to test the registry mechanics in
// isolation from any real backend (gnome-screenshot, X11, Windows,
// etc). Real backends register themselves via build-tag-gated init()
// functions, so depending on which OS `go test` runs on, a different
// subset of real backends may already be in the registry — these tests
// must not depend on that. See withIsolatedRegistry.
type mockCapturer struct {
	name      string
	available bool
	img       image.Image
	err       error
}

func (m *mockCapturer) Name() string    { return m.name }
func (m *mockCapturer) Available() bool { return m.available }
func (m *mockCapturer) CaptureActiveWindow() (image.Image, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.img, nil
}

// withIsolatedRegistry swaps the package-level registry for an empty
// one for the duration of a test, restoring the original (which may
// contain real, build-tag-gated backends) afterward. This is what makes
// registry tests deterministic regardless of which real backends
// happen to be compiled in and Available() on the machine running the
// tests — without this, a test asserting "Detect returns my mock"
// would be flaky on any machine/CI runner where a real backend also
// reports itself available.
func withIsolatedRegistry(t *testing.T) {
	t.Helper()
	original := registry
	registry = nil
	t.Cleanup(func() { registry = original })
}

func TestRegistry_Register_AddsToList(t *testing.T) {
	withIsolatedRegistry(t)
	Register(&mockCapturer{name: "mock-a", available: true})
	Register(&mockCapturer{name: "mock-b", available: false})

	names := List()
	if len(names) != 2 || names[0] != "mock-a" || names[1] != "mock-b" {
		t.Fatalf("expected List() = [mock-a mock-b] in registration order, got %v", names)
	}
}

func TestRegistry_Detect_ReturnsFirstAvailable(t *testing.T) {
	withIsolatedRegistry(t)
	Register(&mockCapturer{name: "unavailable-1", available: false})
	Register(&mockCapturer{name: "unavailable-2", available: false})
	Register(&mockCapturer{name: "available-first", available: true})
	Register(&mockCapturer{name: "available-second", available: true})

	got := Detect()
	if got == nil {
		t.Fatalf("expected Detect() to return a backend, got nil")
	}
	if got.Name() != "available-first" {
		t.Fatalf("expected Detect() to return the FIRST available backend in registration order "+
			"(available-first), got %q — registration order determines priority when multiple backends "+
			"are available, this must not change silently", got.Name())
	}
}

func TestRegistry_Detect_ReturnsNilWhenNoneAvailable(t *testing.T) {
	withIsolatedRegistry(t)
	Register(&mockCapturer{name: "unavailable-1", available: false})
	Register(&mockCapturer{name: "unavailable-2", available: false})

	got := Detect()
	if got != nil {
		t.Fatalf("expected Detect() to return nil when no backend is available, got %q", got.Name())
	}
}

func TestRegistry_Detect_EmptyRegistryReturnsNil(t *testing.T) {
	withIsolatedRegistry(t)
	got := Detect()
	if got != nil {
		t.Fatalf("expected Detect() on an empty registry to return nil, got %q", got.Name())
	}
}

func TestRegistry_Get_ReturnsNamedBackendRegardlessOfAvailability(t *testing.T) {
	// Get is used for explicit SCROLLSHOT_BACKEND overrides — it should
	// find a backend by name even if Available() is currently false, so
	// the caller can inspect it and report a clear "not available"
	// error rather than a generic "not found".
	withIsolatedRegistry(t)
	Register(&mockCapturer{name: "target", available: false})
	Register(&mockCapturer{name: "other", available: true})

	got := Get("target")
	if got == nil {
		t.Fatalf("expected Get(\"target\") to find the backend even though Available()=false, got nil")
	}
	if got.Name() != "target" {
		t.Fatalf("Get(\"target\") returned wrong backend: %q", got.Name())
	}
}

func TestRegistry_Get_UnknownNameReturnsNil(t *testing.T) {
	withIsolatedRegistry(t)
	Register(&mockCapturer{name: "known", available: true})

	got := Get("does-not-exist")
	if got != nil {
		t.Fatalf("expected Get() with an unregistered name to return nil, got %q", got.Name())
	}
}

func TestRegistry_List_EmptyRegistryReturnsEmptyNotNil(t *testing.T) {
	withIsolatedRegistry(t)
	names := List()
	if len(names) != 0 {
		t.Fatalf("expected List() on an empty registry to be empty, got %v", names)
	}
}

func TestMockCapturer_CaptureActiveWindow_PropagatesError(t *testing.T) {
	// Sanity check on the test double itself — if this fails, other
	// tests relying on mockCapturer's error path can't be trusted.
	wantErr := errors.New("simulated capture failure")
	m := &mockCapturer{name: "failing", err: wantErr}
	_, err := m.CaptureActiveWindow()
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected mockCapturer to propagate its configured error, got %v", err)
	}
}

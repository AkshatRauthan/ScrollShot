package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateOutput(t *testing.T) {
	tmp := t.TempDir()

	// Override environment variables to force getHomeDir to return our temp dir
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("LOCALAPPDATA", tmp)
	t.Setenv("APPDATA", tmp)

	filename := "test_output.png"
	f, path, err := CreateOutput(filename)
	if err != nil {
		t.Fatalf("CreateOutput failed: %v", err)
	}
	defer f.Close()

	// Verify it picked the primary candidate: tmp/Pictures/Screenshots
	expectedDir := filepath.Join(tmp, "Pictures", "Screenshots")
	expectedPath := filepath.Join(expectedDir, filename)

	if path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, path)
	}

	// Verify file actually exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file was not created at %q: %v", path, err)
	}
}

func TestIsValidHome(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"", false},
		{filepath.Join("C:", "Users"), false}, // Invalid (missing username)
		{filepath.Join("C:", "Users") + string(filepath.Separator), false},
		{filepath.Join("C:", "Users", "Akshat"), true},
		{filepath.Join("/", "home", "akshat"), true},
		{"/", false},    // rejected because base is root separator
	}

	for _, tc := range tests {
		got := isValidHome(tc.path)
		if got != tc.want {
			t.Errorf("isValidHome(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

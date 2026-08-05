// Package paths provides OS-appropriate directory paths for scrollshot's
// file storage. Centralising these here ensures the CLI, session package,
// and any future subcommands all agree on where to look without each
// independently hard-coding platform assumptions.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"scrollshot/internal/debug"
)

// SessionDir returns the OS-appropriate directory for storing capture
// session frames between `scrollshot capture` and `scrollshot finish`.
//
// The directory sits inside the OS user cache location:
//
//   - Linux:   $XDG_CACHE_HOME (~/.cache by default)
//   - macOS:   ~/Library/Caches
//   - Windows: %LocalAppData% (e.g. C:\Users\User\AppData\Local)
//
// The returned path is always [cache]/scrollshot_session, so the
// Linux result is identical to the previous hardcoded ~/.cache/scrollshot_session.
func SessionDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving user cache directory: %w", err)
	}
	dir := filepath.Join(cache, "scrollshot_session")
	debug.Logf("paths", "session dir: %s", dir)
	return dir, nil
}

// getHomeDir resolves the user's home directory robustly.
// On Windows, if os.UserHomeDir() returns C:\Users (missing username),
// it falls back to %USERPROFILE%, %LOCALAPPDATA%, or %APPDATA%.
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && isValidHome(home) {
		return home
	}
	if u := os.Getenv("USERPROFILE"); isValidHome(u) {
		return u
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		// LocalAppData is C:\Users\Username\AppData\Local
		h := filepath.Dir(filepath.Dir(local))
		if isValidHome(h) {
			return h
		}
	}
	if app := os.Getenv("APPDATA"); app != "" {
		h := filepath.Dir(filepath.Dir(app))
		if isValidHome(h) {
			return h
		}
	}
	return home
}

func isValidHome(path string) bool {
	if path == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(filepath.Clean(path)))
	// "users" means it resolved to C:\Users without a username subfolder
	return base != "users" && base != "\\" && base != "."
}

// CreateOutput tries multiple fallback directories to save the final stitched
// screenshot. It actually attempts to create the target file, which is the
// only reliable way on Windows to verify if a OneDrive/junction directory
// is truly writable, avoiding "The system cannot find the file specified" errors.
func CreateOutput(filename string) (*os.File, string, error) {
	home := getHomeDir()
	debug.Logf("paths", "resolved home dir: %s", home)

	candidates := []string{
		filepath.Join(home, "Pictures", "Screenshots"),
		filepath.Join(home, "Pictures"),
		filepath.Join(home, "scrollshot_output"),
		home,
	}

	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			debug.Logf("paths", "candidate %s failed MkdirAll: %v", dir, err)
			continue
		}

		path := filepath.Join(dir, filename)
		f, err := os.Create(path)
		if err != nil {
			debug.Logf("paths", "candidate %s failed os.Create for %s: %v", dir, filename, err)
			continue
		}

		debug.Logf("paths", "successfully created output file: %s", path)
		return f, path, nil
	}

	return nil, "", fmt.Errorf("could not create output file in any candidate directory under %s", home)
}

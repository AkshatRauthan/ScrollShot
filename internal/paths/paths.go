// Package paths provides OS-appropriate directory paths for scrollshot's
// file storage. Centralising these here ensures the CLI, session package,
// and any future subcommands all agree on where to look without each
// independently hard-coding platform assumptions.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
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
	return filepath.Join(cache, "scrollshot_session"), nil
}

// OutputDir returns the OS-appropriate directory for saving the final
// stitched screenshot, creating it if it does not already exist.
//
//   - Linux:   ~/Pictures/Screenshots
//   - macOS:   ~/Pictures/Screenshots
//   - Windows: %USERPROFILE%\Pictures\Screenshots
func OutputDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	dir := filepath.Join(home, "Pictures", "Screenshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory %s: %w", dir, err)
	}
	return dir, nil
}

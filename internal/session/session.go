// Package session manages the on-disk staging area for a scrolling
// screenshot capture: where frames get saved as you press "capture"
// repeatedly, and how a stale/abandoned session gets detected and
// cleared automatically so you don't need a separate "start" step.
package session

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"time"

	"scrollshot/internal/debug"
	"scrollshot/internal/paths"
)

// StaleGap is how long since the last capture before a new capture call
// assumes you're starting a fresh session (rather than continuing an old,
// never-finished one) and wipes leftover frames automatically.
const StaleGap = 2 * time.Minute

// Session represents the frame-storage area for one capture-to-finish
// cycle. All frames live under ~/.cache/scrollshot_session as
// frame_0000.png, frame_0001.png, etc, in capture order.
type Session struct {
	dir string
}

// New returns a Session pointed at the OS-appropriate cache location.
// The directory is resolved by internal/paths.SessionDir, which maps to
// the correct user cache root on each platform.
func New() (*Session, error) {
	dir, err := paths.SessionDir()
	if err != nil {
		return nil, err
	}
	return &Session{dir: dir}, nil
}

// Dir returns the session's storage directory.
func (s *Session) Dir() string {
	return s.dir
}

// Frames returns the paths of all saved frames, sorted in capture order.
func (s *Session) Frames() ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(s.dir, "frame_*.png"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// EnsureFresh prepares the session directory for a new capture: creates
// it if missing, and — if the most recent frame is older than StaleGap —
// wipes any leftover frames from a previous, never-finished session.
// Returns true if a stale session was cleared, so the caller can print a
// heads-up.
func (s *Session) EnsureFresh() (clearedStale bool, err error) {
	debug.Logf("session", "checking freshness of %s (stale_gap=%v)", s.dir, StaleGap)
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return false, fmt.Errorf("creating session dir: %w", err)
	}

	frames, err := s.Frames()
	if err != nil {
		return false, err
	}
	if len(frames) == 0 {
		return false, nil
	}

	last := frames[len(frames)-1]
	info, err := os.Stat(last)
	if err != nil {
		return false, nil // can't stat it, don't block capture over this
	}
	elapsed := time.Since(info.ModTime())
	if elapsed > StaleGap {
		debug.Logf("session", "session stale (last modification %v ago) — clearing old frames", elapsed.Round(time.Second))
		if err := s.Clear(); err != nil {
			return false, err
		}
		return true, nil
	}
	debug.Logf("session", "session is fresh (last modification %v ago)", elapsed.Round(time.Second))
	return false, nil
}

// SaveFrame writes img as the next frame in sequence and returns its
// index and path.
func (s *Session) SaveFrame(img image.Image) (index int, path string, err error) {
	frames, err := s.Frames()
	if err != nil {
		return 0, "", err
	}
	idx := len(frames)
	debug.Logf("session", "saving frame #%d", idx)
	path = filepath.Join(s.dir, fmt.Sprintf("frame_%04d.png", idx))

	f, err := os.Create(path)
	if err != nil {
		return 0, "", fmt.Errorf("creating frame file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return 0, "", fmt.Errorf("encoding frame: %w", err)
	}
	return idx, path, nil
}

// Clear deletes all frames in the session, leaving the directory ready
// for a new session. Called after a successful finish, and internally
// by EnsureFresh when a stale session is detected.
func (s *Session) Clear() error {
	debug.Logf("session", "clearing session directory %s", s.dir)
	frames, err := s.Frames()
	if err != nil {
		return err
	}
	for _, f := range frames {
		os.Remove(f)
	}
	return nil
}

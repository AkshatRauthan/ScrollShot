package autoscroll

// StopReason explains why an autoscroll session ended.
type StopReason int

const (
	// Configured frame limit reached.
	StopMaxFrames StopReason = iota

	// Consecutive captures stopped producing meaningful new content.
	StopStagnation

	// User interrupted the session.
	StopCancelled

	// Backend failure.
	StopError
)

// String returns a human-readable description of the stop reason.
func (r StopReason) String() string {
	switch r {
	case StopMaxFrames:
		return "maximum frame limit reached"

	case StopStagnation:
		return "scroll stagnation"

	case StopCancelled:
		return "cancellation by user"

	case StopError:
		return "backend error"

	default:
		return "unknown error"
	}
}

// Result summarizes an autoscroll run.
type Result struct {
	FramesCaptured int
	StopReason     StopReason
}

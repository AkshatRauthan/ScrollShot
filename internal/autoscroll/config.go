package autoscroll

import "time"

// Config controls the behaviour of the autoscroll controller.
type Config struct {
	// Fraction of the viewport height to scroll each iteration.
	// Example: 0.90 scrolls roughly 90% of the window height.
	ScrollFraction float64

	// Time to wait after each scroll before capturing the next frame.
	Delay time.Duration

	// Absolute safety limit to prevent infinite capture.
	MaxFrames int

	// Number of consecutive "no movement" detections required before
	// considering the end of the document reached.
	MaxStagnation int
}

// DefaultConfig returns sensible defaults for general desktop use.
func DefaultConfig() Config {
	return Config{
		ScrollFraction: 0.90,
		Delay:          200 * time.Millisecond,
		MaxFrames:      250,
		MaxStagnation:  2,
	}
}
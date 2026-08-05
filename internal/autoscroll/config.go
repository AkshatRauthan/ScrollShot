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

	// Minimum number of newly revealed pixels required for a capture to
	// be considered meaningful movement. Smaller advances count as
	// stagnation.
	MinimumAdvancePx int

	// Number of consecutive stagnating captures required before the
	// controller concludes scrolling has reached the end.
	StagnationLimit int
}

// DefaultConfig returns sensible defaults for general desktop use.
func DefaultConfig() Config {
	return Config{
		ScrollFraction:   0.7,
		Delay:            400 * time.Millisecond,
		MaxFrames:        50,
		MinimumAdvancePx: 16,
		StagnationLimit:  3,
	}
}

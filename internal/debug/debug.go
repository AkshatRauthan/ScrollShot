package debug

import (
	"fmt"
	"os"
)

var enabled = os.Getenv("SCROLLSHOT_DEBUG") != ""

// Enable turns on debug logging globally for the rest of the program execution.
func Enable() {
	enabled = true
}

// Logf prints a formatted message to stderr if SCROLLSHOT_DEBUG is set.
// It automatically prefixes the message with the provided component name.
func Logf(component, format string, args ...any) {
	if enabled {
		prefix := fmt.Sprintf("[%s] ", component)
		fmt.Fprintf(os.Stderr, prefix+format+"\n", args...)
	}
}

// IsEnabled returns true if debug logging is enabled.
func IsEnabled() bool {
	return enabled
}

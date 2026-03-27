// Package log provides a minimal debug logger for bnn.
// Activate with:  BNN_DEBUG=1 bnn apply
package log

import (
	"fmt"
	"os"
)

var enabled = os.Getenv("BNN_DEBUG") == "1"

// Debug prints a debug line to stderr when BNN_DEBUG=1.
// Format follows the same [bnn] prefix convention as error messages.
func Debug(format string, args ...any) {
	if enabled {
		fmt.Fprintf(os.Stderr, "[bnn:debug] "+format+"\n", args...)
	}
}

// Enabled reports whether debug logging is active.
func Enabled() bool { return enabled }

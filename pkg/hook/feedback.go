package hook

import (
	"fmt"
	"os"
	"strconv"
)

const prefix = "⚡ mneme: "

// FormatStderr returns the prefixed message string.
func FormatStderr(msg string) string {
	return prefix + msg
}

// WriteStderr writes a prefixed message to stderr.
func WriteStderr(msg string) {
	fmt.Fprintln(os.Stderr, FormatStderr(msg))
}

// DebugLevel reads MNEME_DEBUG. Returns 0 if unset or invalid.
func DebugLevel() int {
	v := os.Getenv("MNEME_DEBUG")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

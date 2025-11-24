package magetasks

import (
	"errors"
	"os/exec"
	"strings"
)

// IsCommandNotFound checks if the error indicates the command was not found.
// This handles exec.ErrNotFound and platform-specific string fallbacks.
func IsCommandNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	// Fallback string matching for edge cases
	errStr := err.Error()
	if strings.Contains(errStr, "executable file not found") {
		return true
	}
	if strings.Contains(errStr, "no such file or directory") {
		return true
	}
	return false
}

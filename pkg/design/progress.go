// progress.go - Progress-related utilities

package design

import (
	"golang.org/x/term"
	"os"
)

// IsInteractiveTerminal checks if stdout is connected to a terminal.
func IsInteractiveTerminal() bool {
	fd := int(os.Stdout.Fd())
	isTerminal := term.IsTerminal(fd)
	return isTerminal
}

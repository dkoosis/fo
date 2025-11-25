//go:build !unix

package fo

import (
	"os"
	"os/exec"
)

// setProcessGroup is a no-op on non-Unix platforms.
func setProcessGroup(cmd *exec.Cmd) {
	// No process group support on this platform
}

// killProcessGroup sends a signal directly to the process on non-Unix platforms.
func killProcessGroup(cmd *exec.Cmd, sig os.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(sig)
}

// killProcessGroupWithSIGKILL kills the process directly on non-Unix platforms.
func killProcessGroupWithSIGKILL(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// getExitCodeFromError extracts the exit code from an exec.ExitError on non-Unix platforms.
// Uses ProcessState.ExitCode() which is available cross-platform since Go 1.12.
func getExitCodeFromError(exitErr *exec.ExitError) (int, bool) {
	if exitErr.ProcessState != nil {
		return exitErr.ProcessState.ExitCode(), true
	}
	return 0, false
}

// getInterruptSignals returns the signals to listen for on non-Unix platforms.
func getInterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

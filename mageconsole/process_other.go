//go:build !unix

package mageconsole

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

// getExitCodeFromError returns false on non-Unix platforms as WaitStatus is not available.
func getExitCodeFromError(exitErr *exec.ExitError) (int, bool) {
	return 0, false
}

// getInterruptSignals returns the signals to listen for on non-Unix platforms.
func getInterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

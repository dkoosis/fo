//go:build unix

package mageconsole

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup configures the command to run in its own process group.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends a signal to the entire process group.
func killProcessGroup(cmd *exec.Cmd, sig os.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Signal(sig)
	}
	sigVal, ok := sig.(syscall.Signal)
	if !ok {
		return cmd.Process.Signal(sig)
	}
	return syscall.Kill(-pgid, sigVal)
}

// killProcessGroupWithSIGKILL sends SIGKILL to the entire process group.
func killProcessGroupWithSIGKILL(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// getExitCodeFromError extracts the exit code from an exec.ExitError.
func getExitCodeFromError(exitErr *exec.ExitError) (int, bool) {
	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if ok {
		return waitStatus.ExitStatus(), true
	}
	return 0, false
}

// getInterruptSignals returns the signals to listen for on Unix.
func getInterruptSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

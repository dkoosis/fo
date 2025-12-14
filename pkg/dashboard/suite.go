package dashboard

import (
	"context"
	"io"
	"os"

	"golang.org/x/term"
)

// Suite orchestrates multiple tasks with TUI or streaming output.
type Suite struct {
	title string
	specs []TaskSpec
}

// NewSuite creates a new dashboard suite with the given title.
func NewSuite(title string) *Suite {
	return &Suite{title: title}
}

// AddTask adds a task with group, name, and shell command.
func (s *Suite) AddTask(group, name, command string) *Suite {
	s.specs = append(s.specs, TaskSpec{
		Group:   group,
		Name:    name,
		Command: command,
	})
	return s
}

// Run executes all tasks. Uses TUI if stdout is a terminal, otherwise streams.
// Returns error if any task fails.
func (s *Suite) Run(ctx context.Context) error {
	return s.RunWithOutput(ctx, os.Stdout)
}

// RunWithOutput executes all tasks with custom output writer.
func (s *Suite) RunWithOutput(ctx context.Context, w io.Writer) error {
	if len(s.specs) == 0 {
		return nil
	}

	// Check if output is a terminal
	isTTY := false
	if f, ok := w.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	if isTTY {
		code, err := RunDashboard(ctx, s.specs)
		if err != nil {
			return err
		}
		if code != 0 {
			return &SuiteError{ExitCode: code}
		}
		return nil
	}

	code := RunNonTTY(ctx, s.specs, w)
	if code != 0 {
		return &SuiteError{ExitCode: code}
	}
	return nil
}

// SuiteError indicates one or more tasks failed.
type SuiteError struct {
	ExitCode int
}

func (e *SuiteError) Error() string {
	return "one or more tasks failed"
}

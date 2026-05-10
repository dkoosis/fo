package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

var errWatchUsage = errors.New("usage: fo watch [flags] -- <command> [args...]")

// parseWatchArgs splits watch args at the `--` separator and returns the
// child argv. Flags before `--` are ignored here; A.2 will own them.
func parseWatchArgs(args []string) ([]string, error) {
	for i, a := range args {
		if a == "--" {
			rest := args[i+1:]
			if len(rest) == 0 {
				return nil, errWatchUsage
			}
			return rest, nil
		}
	}
	return nil, errWatchUsage
}

// watchLoop invokes runOnce immediately, then re-invokes on each value
// received from triggers. Returns when ctx is canceled or triggers closes.
// Single-flight: runOnce is called synchronously, so a slow run blocks
// further trigger handling until it returns.
func watchLoop(ctx context.Context, runOnce func(), triggers <-chan struct{}) {
	runOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-triggers:
			if !ok {
				return
			}
			runOnce()
		}
	}
}

// runWatch is the entry point for `fo watch -- <command> [args...]`.
// A.1 scope: manual trigger via stdin newlines + SIGINT/SIGTERM cancellation.
// A.2 will replace the trigger source with fsnotify.
func runWatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd, err := parseWatchArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	triggers := stdinTriggers(ctx, stdin)
	var lastCode int
	watchLoop(ctx, func() {
		lastCode = runChildAndRender(ctx, cmd, stdout, stderr)
	}, triggers)
	return lastCode
}

// stdinTriggers emits one struct{} per newline received on r. The returned
// channel is closed when r reaches EOF or ctx is canceled.
func stdinTriggers(ctx context.Context, r io.Reader) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			select {
			case ch <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// runChildAndRender executes cmd, captures its stdout, and renders it
// through fo's existing pipeline. Child stderr passes through to stderr.
// Returns the render exit code; child non-zero exit is normal (e.g. test
// failures) and does not short-circuit rendering.
func runChildAndRender(ctx context.Context, cmd []string, stdout, stderr io.Writer) int {
	if len(cmd) == 0 {
		return 2
	}
	var buf bytes.Buffer
	// G204: cmd is the user-supplied argv after `fo watch -- ...`.
	// Executing arbitrary commands IS the feature; the user is the one typing it.
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...) //nolint:gosec // user-supplied command is the contract
	c.Stdout = &buf
	c.Stderr = stderr
	_ = c.Run() // child non-zero is expected (test failures, lint findings)
	if buf.Len() == 0 {
		return 0
	}
	return run(nil, &buf, stdout, stderr)
}

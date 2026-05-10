package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var errWatchUsage = errors.New("usage: fo watch [flags] -- <command> [args...]")

// watchOpts are flags accepted before `--` in `fo watch`.
type watchOpts struct {
	debounce time.Duration
	source   string // "fs" (default) or "stdin"
}

// parseWatchArgs splits watch args at the `--` separator. Flags before `--`
// configure the watcher; the trailing argv is the child command.
func parseWatchArgs(args []string) ([]string, error) {
	cmd, _, err := parseWatchArgsWithOpts(args)
	return cmd, err
}

// parseWatchArgsWithOpts is the form used by runWatch.
func parseWatchArgsWithOpts(args []string) ([]string, watchOpts, error) {
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 {
		return nil, watchOpts{}, errWatchUsage
	}
	flagArgs := args[:sep]
	cmd := args[sep+1:]
	if len(cmd) == 0 {
		return nil, watchOpts{}, errWatchUsage
	}
	opts := watchOpts{debounce: 250 * time.Millisecond, source: "fs"}
	fs := flag.NewFlagSet("fo watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.DurationVar(&opts.debounce, "debounce", opts.debounce, "coalesce burst events within this window")
	fs.StringVar(&opts.source, "source", opts.source, "trigger source: fs|stdin")
	if err := fs.Parse(flagArgs); err != nil {
		return nil, watchOpts{}, fmt.Errorf("watch: %w", err)
	}
	if opts.source != "fs" && opts.source != "stdin" {
		return nil, watchOpts{}, fmt.Errorf("%w: -source must be fs or stdin", errWatchUsage)
	}
	return cmd, opts, nil
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
	cmd, opts, err := parseWatchArgsWithOpts(args)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var triggers <-chan struct{}
	switch opts.source {
	case "stdin":
		triggers = stdinTriggers(ctx, stdin)
	default: // "fs"
		raw, err := watchTree(ctx, ".")
		if err != nil {
			fmt.Fprintf(stderr, "fo: watch: %v\n", err)
			return 2
		}
		triggers = debounce(ctx, raw, opts.debounce)
	}

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

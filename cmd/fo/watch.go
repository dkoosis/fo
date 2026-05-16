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
// further trigger handling until it returns. between is invoked before
// every rerun (not the initial run) so the caller can repaint a status
// line / clear the screen.
func watchLoop(ctx context.Context, runOnce func(), between func(), triggers <-chan struct{}) {
	runOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-triggers:
			if !ok {
				return
			}
			if between != nil {
				between()
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

	// Keyboard control is offered only for the fs trigger source. The stdin
	// source consumes newlines as triggers and is mutually exclusive with
	// raw-mode keypress reads on the same descriptor.
	var keyTriggers <-chan struct{}
	restoreTTY := func() {}
	if opts.source != "stdin" {
		keyTriggers, restoreTTY = keyControl(ctx, stdin, stop)
	}
	defer restoreTTY()

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
	if keyTriggers != nil {
		triggers = fanIn(ctx, triggers, keyTriggers)
	}

	isTTY := isTTYWriter(stdout)
	var lastCode int
	var runN int
	runOnce := func() {
		runN++
		started := time.Now()
		lastCode = runChildAndRender(ctx, cmd, stdout, stderr)
		writeWatchStatus(stdout, isTTY, runN, started, time.Since(started), lastCode)
	}
	between := func() {
		if isTTY {
			// Cursor home + clear screen. Plain ANSI so we don't pull in a
			// full TUI dep — A.4 minimum. Falls back to a blank line on
			// non-TTY (handled by the !isTTY branch in writeWatchStatus).
			fmt.Fprint(stdout, "\x1b[H\x1b[2J")
		}
	}
	watchLoop(ctx, runOnce, between, triggers)
	return lastCode
}

// writeWatchStatus prints a one-line trailer after each rerun showing
// run-number, completion time, duration, exit code. Trailer-not-header
// keeps it out of the way for piped/non-TTY consumers and avoids hiding
// the render output behind a status bar.
func writeWatchStatus(w io.Writer, isTTY bool, runN int, started time.Time, dur time.Duration, code int) {
	if isTTY {
		fmt.Fprintf(w, "\n— watch · run #%d · %s · %s · exit %d\n",
			runN, started.Format("15:04:05"), dur.Round(time.Millisecond), code)
		return
	}
	fmt.Fprintf(w, "# fo:watch run=%d at=%s dur=%s exit=%d\n",
		runN, started.UTC().Format(time.RFC3339), dur.Round(time.Millisecond), code)
}

// stdinTriggers emits one struct{} per newline received on r. The returned
// channel is closed when r reaches EOF or ctx is canceled.
//
// Scanner buffer is explicitly bounded (1 MiB max line) so a hostile or
// malformed input can't drive unbounded allocation via the default growth path.
//
// A blocked Read can't be interrupted by ctx alone. If r is an io.Closer
// (e.g. *os.File for os.Stdin), we close it on ctx cancel to unblock the
// scanner; for non-closable readers (strings.Reader in tests, pipes we don't
// own) the reader goroutine remains parked until the next byte or EOF — by
// then the process is usually exiting anyway.
func stdinTriggers(ctx context.Context, r io.Reader) <-chan struct{} {
	const maxLine = 1 << 20 // 1 MiB
	ch := make(chan struct{})
	done := make(chan struct{})

	if c, ok := r.(io.Closer); ok {
		go func() {
			select {
			case <-ctx.Done():
				_ = c.Close()
			case <-done:
			}
		}()
	}

	go func() {
		defer close(ch)
		defer close(done)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), maxLine)
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

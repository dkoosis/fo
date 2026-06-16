package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/testjson"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/view"
)

// statePolicy names how the sidecar state writer should be wired.
// Replaces an inverted (noState, stateStrict) bool pair that was
// propagated through 4 signatures and 6 call sites (fo-pii).
type statePolicy int

const (
	// stateOn: load + save sidecar; save failures emit a Notice only.
	stateOn statePolicy = iota
	// stateOff: skip diff classification and sidecar I/O entirely.
	stateOff
	// stateStrict: same as stateOn but a save failure forces exit 2.
	stateStrict
)

// streamOpts bundles the parameters shared by runStream{,Ctx,Batch}.
// One struct instead of an 8-arg signature; one StatePolicy instead of
// an inverted bool pair (fo-pii).
type streamOpts struct {
	stdin     io.Reader
	br        *bufio.Reader
	stdout    io.Writer
	stderr    io.Writer
	theme     theme.Theme
	themeName string // only used by runStreamBatch's deferred renderMode
	mode      string // only used by runStreamBatch
	stateFile string
	policy    statePolicy
}

// runStream pumps go test -json events into per-package Report snapshots and
// hands them to view.RenderStream. One channel send per finished package
// keeps PickView's total-driven thresholds meaningful. Cancellation (SIGINT)
// closes the underlying reader so blocked Reads unblock promptly — fo-op6.
func runStream(opts streamOpts) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runStreamCtx(ctx, opts)
}

// runStreamCtx is runStream's testable core: cancellation root injected.
// Streams events incrementally — never buffers the whole stdin — so large
// CI runs cannot OOM and Ctrl-C exits within the next event boundary.
func runStreamCtx(ctx context.Context, opts streamOpts) int {
	stdin, br, stdout, stderr := opts.stdin, opts.br, opts.stdout, opts.stderr
	t, stateFile := opts.theme, opts.stateFile
	width := termSize(stdout)

	// Load suppression ruleset once for the run so streaming snapshots
	// don't show findings the final summary will then drop (fo-2sk).
	// Stream-time r is nil — Notices for load/parse failures land on
	// the final report via applySuppress below.
	streamRuleset := loadSuppressRuleset(nil, suppressPath(), stderr)

	snapshots := make(chan report.Report, 8)
	// resultCh carries the producer goroutine's terminal state. Using a
	// single struct + blocking receive (a) ensures the producer is fully
	// finished before main inspects parseErr (#267 race), and (b) lets
	// main bound the wait via ctx + grace timeout so a wedged
	// attachDiff/state.Save doesn't deadlock fo (#266).
	type streamResult struct {
		report   *report.Report
		parseErr error
		saveErr  error
	}
	resultCh := make(chan streamResult, 1)

	go func() {
		defer close(snapshots)
		r, parseErr := runTestJSONPipeline(ctx, stdin, br, func(snap report.Report) {
			// Emit a snapshot only at package-finish events. Per-test
			// events would flood RenderStream and PickView. Apply the
			// streaming ruleset so findings don't flicker into the
			// terminal and then disappear in the final summary (fo-2sk).
			if streamRuleset != nil {
				report.ApplyFilter(&snap, streamRuleset, time.Now())
			}
			sendCoalesceSnapshot(ctx, snapshots, snap)
		})
		// Final snapshot with diff attached. Skip state Save on parse
		// error so a partial Report doesn't poison the next run's diff (#262).
		var saveErr error
		if parseErr == nil {
			applySuppress(r, suppressPath(), stderr)
			saveErr = attachDiff(r, stateFile, opts.policy, stderr)
			assignAndPersistIDs(r, opts.policy, stderr)
			recordRun(r, opts.policy, stderr)
		}
		resultCh <- streamResult{report: r, parseErr: parseErr, saveErr: saveErr}
		select {
		case snapshots <- *r:
		case <-ctx.Done():
		}
	}()

	renderErr := view.RenderStream(ctx, stdout, snapshots, t, width)

	// Wait for the producer. If ctx is already done (typical cancel/SIGINT
	// path) give the producer a bounded grace window to finish I/O — long
	// enough for a normal fsync, short enough that a wedged disk doesn't
	// hang fo forever (#266).
	var res streamResult
	select {
	case res = <-resultCh:
	case <-ctx.Done():
		select {
		case res = <-resultCh:
		case <-time.After(2 * time.Second):
			fmt.Fprintln(stderr, "fo: timed out waiting for parser after cancel")
			return 2
		}
	}

	if res.parseErr != nil {
		fmt.Fprintf(stderr, "fo: %v\n", res.parseErr)
		return 2
	}
	if renderErr != nil && !errors.Is(renderErr, context.Canceled) {
		fmt.Fprintf(stderr, "fo: %v\n", renderErr)
		return 2
	}
	if res.saveErr != nil && opts.policy == stateStrict {
		return 2
	}
	return exitCodeReport(res.report)
}

// sendCoalesceSnapshot delivers snap to ch without blocking the parser when
// ch is full. If a slow renderer (or slow stdout writer) leaves stale
// snapshots queued, the oldest one is dropped to make room for the latest.
// Safe with a single producer goroutine. Closes fo-4qh: under rapid
// fan-out (1k packages) with a slow renderer, the parser previously blocked
// on a buffered channel send and delayed both progress and ctx cancellation.
func sendCoalesceSnapshot(ctx context.Context, ch chan report.Report, snap report.Report) {
	select {
	case ch <- snap:
		return
	case <-ctx.Done():
		return
	default:
	}
	// Channel full — drop one stale snapshot (single-producer invariant
	// means no other sender races us) and retry. Worst case the renderer
	// drains in parallel and our send finds an empty slot; either way we
	// never block.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- snap:
	case <-ctx.Done():
	}
}

// runStreamBatch parses go test -json incrementally (so memory never grows
// with input size) but renders a single batch report in the requested mode.
// Used when --stream is set with format=human|llm|json and stdout is not a
// TTY-driving incremental render. Closes fo-frl: piped CI callers can opt
// into streaming and bypass the 256 MiB boundread cap.
func runStreamBatch(opts streamOpts) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r, err := runTestJSONPipeline(ctx, opts.stdin, opts.br, nil)
	if err != nil {
		fmt.Fprintf(opts.stderr, "fo: %v\n", err)
		return 2
	}
	applySuppress(r, suppressPath(), opts.stderr)
	saveErr := attachDiff(r, opts.stateFile, opts.policy, opts.stderr)
	assignAndPersistIDs(r, opts.policy, opts.stderr)
	recordRun(r, opts.policy, opts.stderr)
	if err := renderMode(opts.mode, r, opts.stdout, opts.themeName, nil); err != nil {
		fmt.Fprintf(opts.stderr, "fo: %v\n", err)
		return 2
	}
	if saveErr != nil && opts.policy == stateStrict {
		return 2
	}
	return exitCodeReport(r)
}

// runTestJSONPipeline streams go test -json events from br/stdin into an
// aggregator. onPkgFinish (if non-nil) is invoked at each package terminal
// event with a fresh Report snapshot. Returns the final Report and any
// non-cancel parse error. Honors ctx cancellation by closing stdin when it
// implements io.Closer (so blocked Reads unblock promptly — fo-op6).
func runTestJSONPipeline(ctx context.Context, stdin io.Reader, br *bufio.Reader, onPkgFinish func(report.Report)) (*report.Report, error) {
	if c, ok := stdin.(io.Closer); ok {
		stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })
		defer stopClose()
	}
	// br already wraps stdin and holds the sniffed prefix. Wrap it as a
	// ReadCloser whose Close propagates to stdin (if closable) so
	// testjson.Stream's cancel path unblocks an in-flight Read.
	rc := &bufioReadCloser{Reader: br, closer: closerOf(stdin)}

	agg := testjson.NewAggregator()
	_, err := testjson.Stream(ctx, rc, func(e testjson.TestEvent) {
		agg.ProcessEvent(e)
		if onPkgFinish != nil && e.Test == "" && (e.Action == "pass" || e.Action == "fail" || e.Action == "skip") {
			onPkgFinish(*testjson.ToReport(agg.Results()))
		}
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return testjson.ToReport(agg.Results()), err
	}
	return testjson.ToReport(agg.Results()), nil
}

// bufioReadCloser pairs a *bufio.Reader (carrying the sniffed prefix) with
// the underlying stdin's Close so context-cancel can interrupt blocked
// Reads. closer may be nil for non-closable stdin (tests, pipes).
type bufioReadCloser struct {
	*bufio.Reader
	closer io.Closer
}

func (b *bufioReadCloser) Close() error {
	if b.closer != nil {
		return b.closer.Close()
	}
	return nil
}

func closerOf(r io.Reader) io.Closer {
	if c, ok := r.(io.Closer); ok {
		return c
	}
	return nil
}

// exitCodeReport: 1 if any error finding or non-pass/skip test outcome.
func exitCodeReport(r *report.Report) int {
	if r == nil {
		return 0
	}
	for i := range r.Findings {
		if r.Findings[i].Severity == report.SeverityError {
			return 1
		}
	}
	for i := range r.Tests {
		switch r.Tests[i].Outcome {
		case report.OutcomeFail, report.OutcomePanic, report.OutcomeBuildError:
			return 1
		case report.OutcomePass, report.OutcomeSkip:
			// not a failure
		}
	}
	return 0
}

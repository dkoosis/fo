package view

import (
	"context"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

// RenderReport picks a view from r and writes the rendered string to w.
// Batch arrival mode: caller has the complete Report.
func RenderReport(w io.Writer, r report.Report, t theme.Theme, width int) error {
	return RenderReportMode(w, r, t, width, ModeHuman)
}

// RenderReportMode is RenderReport with an explicit audience mode.
func RenderReportMode(w io.Writer, r report.Report, t theme.Theme, width int, mode Mode) error {
	return RenderReportModeWithExpand(w, r, t, width, mode, expandSet{})
}

// RenderReportModeWithExpand is RenderReportMode plus an --expand set.
// LLM mode ignores the set (clusters always render fully).
func RenderReportModeWithExpand(w io.Writer, r report.Report, t theme.Theme, width int, mode Mode, expand expandSet) error {
	out := Render(PickViewModeWithExpand(r, mode, expand), t, width)
	if out == "" {
		return nil
	}
	_, err := fmt.Fprintln(w, out)
	return err
}

// RenderStream consumes successive Report snapshots from ch and emits a
// fresh PickView+Render per snapshot, separated by blank lines. The
// final snapshot received before ch closes IS the final summary — same
// renderer, same code path as batch. No footer, no terminal-state
// machinery: live mode is just batch repeated.
//
// The choice of report.Report (whole-snapshot) over a per-event channel
// is deliberate: PickView's thresholds are total-driven, so the renderer
// would otherwise re-implement parser accumulation. Parsers stream
// snapshots at natural boundaries (per-package finish for testjson,
// per-run for sarif).
//
// Returns when ch is closed or ctx is cancelled. The caller owns ch.
func RenderStream(ctx context.Context, w io.Writer, ch <-chan report.Report, t theme.Theme, width int) error {
	return RenderStreamMode(ctx, w, ch, t, width, ModeHuman)
}

// RenderStreamMode is RenderStream with an explicit audience mode.
//
// Clean snapshots (no findings, no failures) are heartbeats, not changes:
// a passing package in a passing run flattens the signal the reader is
// scanning for (fo-58k). They are deferred and emitted at most once — at
// end-of-stream, only if no non-clean snapshot ever rendered. Any
// non-clean snapshot discards a pending Clean heartbeat.
func RenderStreamMode(ctx context.Context, w io.Writer, ch <-chan report.Report, t theme.Theme, width int, mode Mode) error {
	first := true
	var pendingClean *report.Report
	rendered := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r, ok := <-ch:
			if !ok {
				return flushStream(w, pendingClean, t, width, &first, mode, rendered)
			}
			next, err := handleSnapshot(w, r, t, width, &first, mode, pendingClean)
			if err != nil {
				return err
			}
			pendingClean = next.pending
			if next.rendered {
				rendered = true
			}
		}
	}
}

type streamStep struct {
	pending  *report.Report
	rendered bool
}

func handleSnapshot(w io.Writer, r report.Report, t theme.Theme, width int, first *bool, mode Mode, pending *report.Report) (streamStep, error) {
	if _, isClean := PickViewMode(r, mode).(Clean); isClean {
		snap := r
		return streamStep{pending: &snap}, nil
	}
	_ = pending // drop pending Clean
	if err := writeSnapshot(w, r, t, width, first, mode); err != nil {
		return streamStep{}, err
	}
	return streamStep{rendered: true}, nil
}

func flushStream(w io.Writer, pendingClean *report.Report, t theme.Theme, width int, first *bool, mode Mode, rendered bool) error {
	if pendingClean != nil && !rendered {
		return writeSnapshot(w, *pendingClean, t, width, first, mode)
	}
	return nil
}

// writeSnapshot renders one report snapshot and writes it to w, prepending a
// blank separator line for all but the first snapshot.
func writeSnapshot(w io.Writer, r report.Report, t theme.Theme, width int, first *bool, mode Mode) error {
	out := Render(PickViewMode(r, mode), t, width)
	if out == "" {
		return nil
	}
	if !*first {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	*first = false
	_, err := fmt.Fprintln(w, out)
	return err
}

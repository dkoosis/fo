package view_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/view"
)

// sampleReport is a small Report that picks Bullet (count too low for
// Leaderboard/Grouped, no panic, not clean).
func sampleReport() report.Report {
	return report.Report{
		Tool: "test",
		Findings: []report.Finding{
			{RuleID: "X1", File: "a.go", Line: 1, Severity: report.SeverityError, Message: "boom"},
			{RuleID: "X2", File: "b.go", Line: 2, Severity: report.SeverityWarning, Message: "warn"},
		},
	}
}

// TestRenderStream_FinalEqualsBatch — streaming with one final snapshot
// produces the same output as RenderReport on that snapshot.
func TestRenderStream_FinalEqualsBatch(t *testing.T) {
	r := sampleReport()
	tm := theme.Mono()

	var batch bytes.Buffer
	if err := view.RenderReport(&batch, r, tm, 80); err != nil {
		t.Fatalf("batch: %v", err)
	}

	var streamBuf bytes.Buffer
	ch := make(chan report.Report, 1)
	ch <- r
	close(ch)
	if err := view.RenderStream(context.Background(), &streamBuf, ch, tm, 80); err != nil {
		t.Fatalf("stream: %v", err)
	}

	if batch.String() != streamBuf.String() {
		t.Errorf("stream != batch:\nbatch=%q\nstream=%q", batch.String(), streamBuf.String())
	}
}

// progressWriter records the byte count of every Write call so a test
// can verify writes happened across multiple ticks rather than one
// flush at end.
type progressWriter struct {
	mu      sync.Mutex
	writes  []int
	content bytes.Buffer
	signal  chan struct{}
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writes = append(p.writes, len(b))
	n, err := p.content.Write(b)
	if p.signal != nil {
		select {
		case p.signal <- struct{}{}:
		default:
		}
	}
	return n, err
}

func (p *progressWriter) Writes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.writes)
}

// TestRenderStream_IncrementalEmission — feeding multiple snapshots
// produces multiple writer events as they arrive, not a single
// end-of-stream flush.
func TestRenderStream_IncrementalEmission(t *testing.T) {
	pw := &progressWriter{signal: make(chan struct{}, 8)}
	ch := make(chan report.Report)

	done := make(chan error, 1)
	go func() {
		done <- view.RenderStream(context.Background(), pw, ch, theme.Mono(), 80)
	}()

	// First snapshot
	ch <- sampleReport()
	waitFor(t, pw.signal)
	first := pw.Writes()
	if first == 0 {
		t.Fatal("no writes after first snapshot")
	}

	// Second snapshot — must produce more writes BEFORE the channel closes.
	r2 := sampleReport()
	r2.Findings = append(r2.Findings, report.Finding{
		RuleID: "X3", File: "c.go", Line: 3,
		Severity: report.SeverityNote, Message: "note",
	})
	ch <- r2
	waitFor(t, pw.signal)
	second := pw.Writes()
	if second <= first {
		t.Errorf("second snapshot did not produce additional writes: first=%d second=%d", first, second)
	}

	close(ch)
	if err := <-done; err != nil {
		t.Fatalf("stream: %v", err)
	}
}

func waitFor(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for write")
	}
}

// TestRenderStream_EmptyClose — closing an empty channel produces no
// output and no error.
func TestRenderStream_EmptyClose(t *testing.T) {
	ch := make(chan report.Report)
	close(ch)
	var buf bytes.Buffer
	if err := view.RenderStream(context.Background(), &buf, ch, theme.Mono(), 80); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

// TestRenderStream_CleanReport — a clean Report renders the empty-state
// view, both via batch and stream, identically.
func TestRenderStream_CleanReport(t *testing.T) {
	r := report.Report{Tool: "test"}
	tm := theme.Mono()

	var batch bytes.Buffer
	if err := view.RenderReport(&batch, r, tm, 80); err != nil {
		t.Fatalf("batch: %v", err)
	}

	ch := make(chan report.Report, 1)
	ch <- r
	close(ch)
	var streamBuf bytes.Buffer
	if err := view.RenderStream(context.Background(), &streamBuf, ch, tm, 80); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if batch.String() != streamBuf.String() {
		t.Errorf("clean stream != batch:\nbatch=%q\nstream=%q", batch.String(), streamBuf.String())
	}
	if !strings.Contains(batch.String(), "no findings") {
		t.Errorf("expected 'no findings', got %q", batch.String())
	}
}

// TestRenderStream_ContextCancel — cancelling ctx returns ctx.Err()
// without leaking the goroutine.
func TestRenderStream_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan report.Report) // never sent on
	done := make(chan error, 1)
	go func() {
		done <- view.RenderStream(ctx, io.Discard, ch, theme.Mono(), 80)
	}()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected ctx error")
		}
	case <-time.After(time.Second):
		t.Fatal("RenderStream did not return after cancel")
	}
}

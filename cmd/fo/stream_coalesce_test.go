package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

// TestSendCoalesceSnapshot_DoesNotBlock verifies the parser never stalls
// when the renderer stops draining the channel. Regression for fo-4qh.
func TestSendCoalesceSnapshot_DoesNotBlock(t *testing.T) {
	ctx := t.Context()
	ch := make(chan report.Report, 2)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Send far more than the channel capacity with no consumer.
		for i := range 100 {
			snap := report.Report{Tool: fmt.Sprintf("snap-%d", i)}
			sendCoalesceSnapshot(ctx, ch, snap)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sendCoalesceSnapshot blocked despite full channel and no consumer")
	}

	// Drain the channel and confirm only the last snap survived (or one
	// of the most recent — order is preserved, but earlier values are
	// dropped). Latest snap should be present.
	var latest report.Report
	for {
		select {
		case v := <-ch:
			latest = v
		default:
			if latest.Tool != "snap-99" {
				t.Errorf("latest snap = %q, want snap-99", latest.Tool)
			}
			return
		}
	}
}

// TestSendCoalesceSnapshot_RespectsCancel verifies that a cancelled context
// stops the helper even when the channel is permanently full.
func TestSendCoalesceSnapshot_RespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan report.Report, 1)
	ch <- report.Report{} // pre-fill

	cancel()
	done := make(chan struct{})
	go func() {
		// The helper drains the slot then attempts to send. If the receive
		// branch fires first, the next send may succeed, but it must not
		// block on a non-existent consumer.
		sendCoalesceSnapshot(ctx, ch, report.Report{Tool: "x"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendCoalesceSnapshot did not return after ctx cancel")
	}
}

// TestRunStreamCtx_SlowWriterDoesNotStallParser feeds a burst of
// package-finish events and a slow stdout writer, then verifies the whole
// run completes within bounded time. Without coalescing, the parser
// blocks on the bounded channel under render back-pressure and total
// runtime scales with packageCount × writerLatency. With coalescing, the
// parser tears through the input in milliseconds while the renderer
// catches up on a few snapshots.
func TestRunStreamCtx_SlowWriterDoesNotStallParser(t *testing.T) {
	const packages = 200
	const writeDelay = 30 * time.Millisecond

	var input bytes.Buffer
	for p := range packages {
		fmt.Fprintf(&input,
			`{"Time":"2026-04-29T00:00:00Z","Action":"pass","Package":"example.com/p%03d","Elapsed":0.01}`+"\n",
			p)
	}

	stdin := io.NopCloser(&input)
	br := bufio.NewReaderSize(stdin, 8*1024)
	stdout := &slowWriter{delay: writeDelay}
	var stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	start := time.Now()
	rc := runStreamCtx(ctx, stdin, br, stdout, theme.Mono(), "", true, false, &stderr)
	elapsed := time.Since(start)

	// If the parser were blocking on the bounded channel under back-pressure
	// from the slow writer, total time would approach packages*writeDelay
	// (200 * 30ms = 6s). Coalescing keeps it well under that bound.
	if elapsed > 3*time.Second {
		t.Fatalf("runStreamCtx took %v with slow writer; coalescing send is not preventing the stall", elapsed)
	}
	if rc != 0 {
		t.Errorf("rc=%d stderr=%q", rc, stderr.String())
	}
}

// slowWriter delays each Write to model a renderer or terminal that
// drains snapshots slowly.
type slowWriter struct {
	delay time.Duration
	buf   bytes.Buffer
}

func (s *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(s.delay)
	return s.buf.Write(p)
}

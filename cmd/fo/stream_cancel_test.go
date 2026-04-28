package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/theme"
)

// slowProducer emits a few well-formed go test -json events, then blocks
// on Read until ctx is cancelled or its Close is called. Models a long
// `go test -json` stream that the user interrupts with Ctrl-C.
type slowProducer struct {
	ctx    context.Context //nolint:containedctx // test-only fixture
	prefix *bytes.Reader
	closed chan struct{}
}

func newSlowProducer(ctx context.Context, prefix []byte) *slowProducer {
	return &slowProducer{
		ctx:    ctx,
		prefix: bytes.NewReader(prefix),
		closed: make(chan struct{}),
	}
}

func (s *slowProducer) Read(p []byte) (int, error) {
	if s.prefix.Len() > 0 {
		return s.prefix.Read(p)
	}
	// Block until cancel or Close.
	select {
	case <-s.ctx.Done():
		return 0, io.EOF
	case <-s.closed:
		return 0, io.EOF
	}
}

func (s *slowProducer) Close() error {
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	return nil
}

// TestRunStream_PromptCancel asserts that runStream exits within a small
// budget after its context is cancelled, even when stdin is still open
// and producing no further data. Regression guard for fo-op6: the old
// io.ReadAll path stalled cancellation until the upstream closed.
func TestRunStream_PromptCancel(t *testing.T) {
	t.Parallel()

	// A few valid events so streaming starts; then the producer blocks.
	events := strings.Join([]string{
		`{"Time":"2026-04-27T12:00:00Z","Action":"run","Package":"foo","Test":"TestA"}`,
		`{"Time":"2026-04-27T12:00:01Z","Action":"pass","Package":"foo","Test":"TestA","Elapsed":0.01}`,
		`{"Time":"2026-04-27T12:00:01Z","Action":"pass","Package":"foo","Elapsed":0.01}`,
	}, "\n") + "\n"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prod := newSlowProducer(ctx, []byte(events))

	var stdout, stderr bytes.Buffer
	br := bufio.NewReaderSize(prod, 8*1024)

	done := make(chan int, 1)
	go func() {
		done <- runStreamCtx(ctx, prod, br, &stdout, theme.Mono(), "", true, &stderr)
	}()

	// Give the streamer a moment to consume the initial events.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()

	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Fatalf("runStream took %v to honor cancel; want <500ms", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runStream did not return within 2s after cancel")
	}
}

// TestRunStream_BoundedNonStreamPath asserts the non-stream path refuses
// inputs larger than the boundread cap, surfacing the cap rather than
// silently OOM-ing. Uses run() directly (format=llm forces buffered path).
func TestRun_NonStreamPathBounded(t *testing.T) {
	t.Parallel()
	// Generate >256 MiB of pseudo-input fast: a giant single line of '{'
	// padding that won't match any sniffer but stresses the cap. We use
	// an io.Reader that lazily produces bytes — no allocation up front.
	const cap256 = 256 << 20
	// Produce slightly more than cap.
	r := io.LimitReader(constReader('a'), cap256+1024)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm", "--no-state"}, r, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2 for oversize input; got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exceeds maximum size") &&
		!strings.Contains(stderr.String(), "input exceeds") {
		// boundread.ErrInputTooLarge wrapping
		if !strings.Contains(stderr.String(), "256") {
			t.Errorf("stderr should mention oversize input; got %q", stderr.String())
		}
	}
}

// constReader produces an infinite stream of a constant byte. Cheap.
type constReader byte

func (c constReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(c)
	}
	return len(p), nil
}


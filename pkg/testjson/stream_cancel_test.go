package testjson

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// blockingCloser blocks Read until Close is called, then returns io.EOF.
// Used to verify that Stream's cancel path actually invokes Close to unblock
// the scanner goroutine (fo-u2w).
type blockingCloser struct {
	closed   chan struct{}
	closeCnt atomic.Int32
}

func newBlockingCloser() *blockingCloser {
	return &blockingCloser{closed: make(chan struct{})}
}

func (b *blockingCloser) Read(p []byte) (int, error) {
	<-b.closed
	return 0, io.EOF
}

func (b *blockingCloser) Close() error {
	if b.closeCnt.Add(1) == 1 {
		close(b.closed)
	}
	return nil
}

// TestStream_CancelClosesReader verifies that cancelling the context invokes
// r.Close(), which is the only mechanism that can unblock a Read that is
// stuck inside the scanner goroutine. Regression for fo-u2w.
func TestStream_CancelClosesReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := newBlockingCloser()

	done := make(chan error, 1)
	go func() {
		_, err := Stream(ctx, r, func(TestEvent) {})
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Stream returned nil error after cancel; expected ctx.Err()")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stream did not return within 2s after cancel — Close was not called or scanner is leaked")
	}

	if got := r.closeCnt.Load(); got == 0 {
		t.Fatal("expected Close to be called on cancel; was not")
	}
}

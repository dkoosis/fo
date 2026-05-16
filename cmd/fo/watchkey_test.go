package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestKeyControl_NonTTYReturnsNoop(t *testing.T) {
	// strings.Reader is not an *os.File → must fall back cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, restore := keyControl(ctx, strings.NewReader(""), cancel)
	if ch != nil {
		t.Fatalf("keyControl: want nil channel on non-TTY, got %v", ch)
	}
	if restore == nil {
		t.Fatal("keyControl: restore must never be nil")
	}
	// Restore must be idempotent and safe to call multiple times.
	restore()
	restore()
}

func TestFanIn_BothClose(t *testing.T) {
	a := make(chan struct{})
	b := make(chan struct{})
	close(a)
	close(b)
	out := fanIn(context.Background(), a, b)
	// Drain — should close promptly.
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("fanIn: unexpected value from closed inputs")
		}
	case <-time.After(time.Second):
		t.Fatal("fanIn: did not close after both inputs closed")
	}
}

func TestFanIn_NilPassthrough(t *testing.T) {
	a := make(chan struct{}, 1)
	a <- struct{}{}
	close(a)

	out := fanIn(context.Background(), a, nil)
	// fanIn(a, nil) is just a (no goroutine, same channel).
	got := 0
	for range out {
		got++
	}
	if got != 1 {
		t.Fatalf("fanIn nil passthrough: want 1 value, got %d", got)
	}
}

func TestFanIn_MergesValues(t *testing.T) {
	a := make(chan struct{}, 2)
	b := make(chan struct{}, 2)
	a <- struct{}{}
	b <- struct{}{}
	a <- struct{}{}
	close(a)
	close(b)

	out := fanIn(context.Background(), a, b)
	got := 0
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				if got != 3 {
					t.Fatalf("fanIn: want 3 merged values, got %d", got)
				}
				return
			}
			got++
		case <-deadline:
			t.Fatalf("fanIn: timed out after %d values", got)
		}
	}
}

func TestFanIn_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := make(chan struct{})
	b := make(chan struct{})
	out := fanIn(ctx, a, b)
	cancel()
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("fanIn: unexpected value after ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("fanIn: did not close after ctx cancel")
	}
}

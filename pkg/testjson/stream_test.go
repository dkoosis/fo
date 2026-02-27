package testjson

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStream_CallsFuncForEachEvent(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestFoo"}`,
		`{"Action":"pass","Package":"example.com/pkg","Test":"TestFoo","Elapsed":0.01}`,
		`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
	}, "\n") + "\n"

	var events []TestEvent
	malformed, err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if malformed != 0 {
		t.Errorf("got %d malformed, want 0", malformed)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	if events[0].Action != "start" {
		t.Errorf("events[0].Action = %q, want \"start\"", events[0].Action)
	}
	if events[2].Test != "TestFoo" {
		t.Errorf("events[2].Test = %q, want \"TestFoo\"", events[2].Test)
	}
}

func TestStream_SkipsMalformedLines(t *testing.T) {
	input := `not json
{"Action":"start","Package":"example.com/pkg"}
also not json
{"Action":"pass","Package":"example.com/pkg","Elapsed":0.1}
`
	var events []TestEvent
	malformed, err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if malformed != 2 {
		t.Errorf("got %d malformed, want 2", malformed)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (malformed lines skipped)", len(events))
	}
}

func TestStream_RespectsContextCancellation(t *testing.T) {
	input := `{"Action":"start","Package":"example.com/pkg"}` + "\n"

	ctx, cancel := context.WithCancel(context.Background())
	var count int
	_, err := Stream(ctx, strings.NewReader(input), func(_ TestEvent) {
		count++
		cancel()
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Stream() unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d events, want 1", count)
	}
}

// blockingReader never returns from Read, simulating a stalled stdin.
type blockingReader struct {
	done chan struct{}
}

func (b *blockingReader) Read([]byte) (int, error) {
	<-b.done
	return 0, io.EOF
}

func (b *blockingReader) Close() error {
	select {
	case <-b.done:
	default:
		close(b.done)
	}
	return nil
}

func TestStream_CancelUnblocksBlockedReader(t *testing.T) {
	br := &blockingReader{done: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := Stream(ctx, br, func(_ TestEvent) {})
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected DeadlineExceeded, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stream did not return after context cancellation — blocked on reader")
	}
}

func TestStream_MalformedCountReturned(t *testing.T) {
	// Mix of valid and invalid NDJSON with a corrupted fail event.
	input := strings.Join([]string{
		`{"Action":"run","Package":"x","Test":"T1"}`,
		`{CORRUPTED}`,
		`{"Action":"fail","Package":"x","Test":"T1","Elapsed":0.1}`,
		`not-json-at-all`,
		`{"Action":"fail","Package":"x","Elapsed":0.2}`,
	}, "\n") + "\n"

	var events []TestEvent
	malformed, err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if malformed != 2 {
		t.Errorf("got %d malformed, want 2", malformed)
	}
	if len(events) != 3 {
		t.Errorf("got %d events, want 3", len(events))
	}
}

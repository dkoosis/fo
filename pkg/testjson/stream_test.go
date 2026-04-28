package testjson

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStream_EventDeliveryAndMalformedCounting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantMalformed int
		wantEvents    int
		check         func(t *testing.T, events []TestEvent)
	}{
		{
			name: "valid stream emits all events in order",
			input: strings.Join([]string{
				`{"Action":"start","Package":"example.com/pkg"}`,
				`{"Action":"run","Package":"example.com/pkg","Test":"TestFoo"}`,
				`{"Action":"pass","Package":"example.com/pkg","Test":"TestFoo","Elapsed":0.01}`,
				`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
			}, "\n") + "\n",
			wantMalformed: 0,
			wantEvents:    4,
			check: func(t *testing.T, events []TestEvent) {
				t.Helper()
				if events[0].Action != "start" {
					t.Fatalf("events[0].Action = %q, want start", events[0].Action)
				}
				if events[2].Test != "TestFoo" {
					t.Fatalf("events[2].Test = %q, want TestFoo", events[2].Test)
				}
			},
		},
		{
			name: "malformed lines are skipped",
			input: `not json
{"Action":"start","Package":"example.com/pkg"}
also not json
{"Action":"pass","Package":"example.com/pkg","Elapsed":0.1}
`,
			wantMalformed: 2,
			wantEvents:    2,
		},
		{
			name: "mixed malformed and valid lines",
			input: strings.Join([]string{
				`{"Action":"run","Package":"x","Test":"T1"}`,
				`{CORRUPTED}`,
				`{"Action":"fail","Package":"x","Test":"T1","Elapsed":0.1}`,
				`not-json-at-all`,
				`{"Action":"fail","Package":"x","Elapsed":0.2}`,
			}, "\n") + "\n",
			wantMalformed: 2,
			wantEvents:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []TestEvent
			malformed, err := Stream(context.Background(), io.NopCloser(strings.NewReader(tt.input)), func(e TestEvent) {
				events = append(events, e)
			})
			if err != nil {
				t.Fatalf("Stream() error: %v", err)
			}
			if malformed != tt.wantMalformed {
				t.Fatalf("malformed = %d, want %d", malformed, tt.wantMalformed)
			}
			if len(events) != tt.wantEvents {
				t.Fatalf("events = %d, want %d", len(events), tt.wantEvents)
			}
			if tt.check != nil {
				tt.check(t, events)
			}
		})
	}
}

func TestStream_RespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var count int
	_, err := Stream(ctx, io.NopCloser(strings.NewReader(`{"Action":"start","Package":"example.com/pkg"}`+"\n")), func(_ TestEvent) {
		count++
		cancel()
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Stream() unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("events processed = %d, want 1", count)
	}
}

func TestStream_PreCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var count int
	_, err := Stream(ctx, io.NopCloser(strings.NewReader(`{"Action":"run","Package":"x","Test":"T"}`+"\n")), func(_ TestEvent) {
		count++
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }
func (r *errReader) Close() error             { return nil }

var errStreamReaderBoom = errors.New("stream reader boom")

func TestStream_PropagatesReaderError(t *testing.T) {
	t.Parallel()

	_, err := Stream(context.Background(), &errReader{err: errStreamReaderBoom}, func(_ TestEvent) {})
	if !errors.Is(err, errStreamReaderBoom) {
		t.Fatalf("err = %v, want errors.Is(errStreamReaderBoom)", err)
	}
}

func TestStream_EmptyInputIsNoop(t *testing.T) {
	t.Parallel()

	var count int
	malformed, err := Stream(context.Background(), io.NopCloser(strings.NewReader("")), func(_ TestEvent) {
		count++
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if malformed != 0 || count != 0 {
		t.Fatalf("malformed=%d count=%d, want 0/0", malformed, count)
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
	t.Parallel()

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
		t.Fatal("Stream did not return after context cancellation")
	}
}

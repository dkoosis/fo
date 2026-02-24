package testjson

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStream_CallsFuncForEachEvent(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestFoo"}`,
		`{"Action":"pass","Package":"example.com/pkg","Test":"TestFoo","Elapsed":0.01}`,
		`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
	}, "\n") + "\n"

	var events []TestEvent
	err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
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
	err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (malformed lines skipped)", len(events))
	}
}

func TestStream_RespectsContextCancellation(t *testing.T) {
	input := `{"Action":"start","Package":"example.com/pkg"}` + "\n"

	ctx, cancel := context.WithCancel(context.Background())
	var count int
	err := Stream(ctx, strings.NewReader(input), func(_ TestEvent) {
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

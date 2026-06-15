package scene_test

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/scene"
)

// decodeCast splits an asciinema v2 stream into its header line and the
// remaining event lines, failing the test on malformed JSON.
func decodeCast(t *testing.T, out string) (header map[string]any, events [][]any) {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if !sc.Scan() {
		t.Fatal("no header line")
	}
	if err := json.Unmarshal(sc.Bytes(), &header); err != nil {
		t.Fatalf("header: %v", err)
	}
	for sc.Scan() {
		var ev []any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("event: %v", err)
		}
		events = append(events, ev)
	}
	return header, events
}

func TestEncodeAsciicast_headerAndEvents(t *testing.T) {
	frames := []scene.Frame{
		{Delay: time.Second, Content: "hello\nworld"},
		{Delay: 500 * time.Millisecond, Content: "hello\nworld\nagain"},
	}
	var b strings.Builder
	if err := scene.EncodeAsciicast(&b, frames); err != nil {
		t.Fatalf("encode: %v", err)
	}

	header, events := decodeCast(t, b.String())

	if header["version"] != float64(2) {
		t.Errorf("version: want 2, got %v", header["version"])
	}
	// width = widest line ("hello"/"world"/"again" = 5), height = tallest
	// frame (3 lines).
	if header["width"] != float64(5) {
		t.Errorf("width: want 5, got %v", header["width"])
	}
	if header["height"] != float64(3) {
		t.Errorf("height: want 3, got %v", header["height"])
	}

	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	// Timestamps are cumulative sums of the delays.
	if events[0][0] != float64(1) {
		t.Errorf("event 0 time: want 1, got %v", events[0][0])
	}
	if events[1][0] != float64(1.5) {
		t.Errorf("event 1 time: want 1.5, got %v", events[1][0])
	}
	// Each event is an "o" output event whose data clears the screen then
	// repaints the full frame (full-redraw model).
	for i, ev := range events {
		if ev[1] != "o" {
			t.Errorf("event %d code: want o, got %v", i, ev[1])
		}
		data, _ := ev[2].(string)
		if !strings.HasPrefix(data, "\x1b[2J\x1b[H") {
			t.Errorf("event %d data not a full redraw: %q", i, data)
		}
		if !strings.Contains(data, frames[i].Content) {
			t.Errorf("event %d data missing frame content", i)
		}
	}
}

// TestEncodeAsciicast_timestampsNoFloatDrift verifies cumulative timestamps
// accumulate without float-summation noise. Summing Delay.Seconds() directly
// would surface 9.9 as 9.899999999999999; accumulating in time.Duration keeps
// it clean.
func TestEncodeAsciicast_timestampsNoFloatDrift(t *testing.T) {
	d := func(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
	frames := []scene.Frame{
		{Delay: d(2.5), Content: "a"},
		{Delay: d(2.5), Content: "b"},
		{Delay: d(1.2), Content: "c"},
		{Delay: d(2.5), Content: "d"},
		{Delay: d(1.2), Content: "e"}, // cumulative 9.9 — the drift trap
	}
	var b strings.Builder
	if err := scene.EncodeAsciicast(&b, frames); err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, events := decodeCast(t, b.String())
	want := []float64{2.5, 5, 6.2, 8.7, 9.9}
	for i, w := range want {
		if events[i][0] != w {
			t.Errorf("event %d time: want %v, got %v", i, w, events[i][0])
		}
	}
}

// TestEncodeAsciicast_stripsANSIForWidth verifies geometry is measured on
// visible width, not raw bytes — color codes must not inflate width.
func TestEncodeAsciicast_stripsANSIForWidth(t *testing.T) {
	frames := []scene.Frame{
		{Content: "\x1b[31mred\x1b[0m"}, // visible width 3
	}
	var b strings.Builder
	if err := scene.EncodeAsciicast(&b, frames); err != nil {
		t.Fatalf("encode: %v", err)
	}
	header, _ := decodeCast(t, b.String())
	if header["width"] != float64(3) {
		t.Errorf("width: want 3 (ANSI stripped), got %v", header["width"])
	}
}

// TestEncodeAsciicast_empty yields a valid header with floor geometry and
// no events.
func TestEncodeAsciicast_empty(t *testing.T) {
	var b strings.Builder
	if err := scene.EncodeAsciicast(&b, nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	header, events := decodeCast(t, b.String())
	if header["width"] != float64(1) || header["height"] != float64(1) {
		t.Errorf("empty geometry: want 1x1, got %vx%v", header["width"], header["height"])
	}
	if len(events) != 0 {
		t.Errorf("want 0 events, got %d", len(events))
	}
}

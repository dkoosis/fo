package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

const echoCmd = "echo"

func TestParseWatchArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd []string
		wantErr bool
	}{
		{"empty", nil, nil, true},
		{"no separator", []string{echoCmd, "hi"}, nil, true},
		{"separator only", []string{"--"}, nil, true},
		{"basic", []string{"--", echoCmd, "hi"}, []string{echoCmd, "hi"}, false},
		{"flag before separator", []string{"-debounce=200ms", "--", "go", "test", "./..."}, []string{"go", "test", "./..."}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWatchArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseWatchArgs(%v): want error, got nil (cmd=%v)", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWatchArgs(%v): unexpected error %v", tt.args, err)
			}
			if !equalSlice(got, tt.wantCmd) {
				t.Fatalf("parseWatchArgs(%v): got %v, want %v", tt.args, got, tt.wantCmd)
			}
		})
	}
}

func TestWatchLoop_RunsInitiallyAndPerTrigger(t *testing.T) {
	triggers := make(chan struct{}, 2)
	triggers <- struct{}{}
	triggers <- struct{}{}
	close(triggers)

	var calls int
	watchLoop(context.Background(), func() { calls++ }, triggers)

	if calls != 3 {
		t.Fatalf("watchLoop: want 3 calls (initial + 2 triggers), got %d", calls)
	}
}

func TestWatchLoop_ExitsOnCtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	triggers := make(chan struct{})

	var calls int
	done := make(chan struct{})
	go func() {
		watchLoop(ctx, func() { calls++ }, triggers)
		close(done)
	}()

	// Wait for initial call to land.
	deadline := time.After(time.Second)
	for calls == 0 {
		select {
		case <-deadline:
			t.Fatal("watchLoop: initial call never observed")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchLoop: did not exit on ctx cancel")
	}
	if calls != 1 {
		t.Fatalf("watchLoop: want exactly 1 call after cancel, got %d", calls)
	}
}

func TestRunChildAndRender_RendersChildStdout(t *testing.T) {
	// Child emits a single go test -json event with one PASS test.
	// fo's pipeline should sniff it as testjson and render it.
	const event = `{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"x","Test":"TestA","Elapsed":0.01}` + "\n" +
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"x","Elapsed":0.01}` + "\n"

	var stdout, stderr bytes.Buffer
	cmd := []string{"sh", "-c", "printf '%s' " + shellQuote(event)}

	code := runChildAndRender(context.Background(), cmd, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("runChildAndRender: want exit 0 (all PASS), got %d (stderr=%q)", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatalf("runChildAndRender: empty stdout, expected rendered output (stderr=%q)", stderr.String())
	}
}

func TestRunChildAndRender_FailingTestExitsNonZero(t *testing.T) {
	const event = `{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"x","Test":"TestA","Elapsed":0.01}` + "\n" +
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"x","Elapsed":0.01}` + "\n"

	var stdout, stderr bytes.Buffer
	cmd := []string{"sh", "-c", "printf '%s' " + shellQuote(event) + "; exit 1"}

	code := runChildAndRender(context.Background(), cmd, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("runChildAndRender: want non-zero exit on test failure, got 0 (stdout=%q stderr=%q)", stdout.String(), stderr.String())
	}
}

func TestRunChildAndRender_EmptyChildOutputIsClean(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := []string{"sh", "-c", "true"}
	code := runChildAndRender(context.Background(), cmd, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runChildAndRender: empty child output should exit 0, got %d", code)
	}
}

func TestRunWatch_MissingSeparatorReturnsError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWatch([]string{echoCmd, "hi"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("runWatch: want non-zero exit, got 0")
	}
	if !strings.Contains(stderr.String(), "watch") {
		t.Fatalf("runWatch: stderr should mention usage, got %q", stderr.String())
	}
}

func TestRunWatch_RunsOnceAndExitsOnStdinEOF(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Empty stdin → triggers closes immediately after initial run.
	// `true` produces no output → render is a no-op.
	code := runWatch([]string{"--", "true"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWatch: want exit 0, got %d (stderr=%q)", code, stderr.String())
	}
}

func TestRunWatch_RerunsOnStdinNewline(t *testing.T) {
	dir := t.TempDir()
	tally := dir + "/n"
	// Each run appends one byte to tally. After watch exits, count bytes.
	cmd := []string{"sh", "-c", "printf x >> " + tally}

	var stdout, stderr bytes.Buffer
	// Two newlines on stdin → initial + 2 reruns = 3 total.
	code := runWatch([]string{"--", cmd[0], cmd[1], cmd[2]},
		strings.NewReader("\n\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWatch: want exit 0, got %d (stderr=%q)", code, stderr.String())
	}

	// Read tally file.
	data, err := os.ReadFile(tally)
	if err != nil {
		t.Fatalf("read tally: %v", err)
	}
	if string(data) != "xxx" {
		t.Fatalf("watch should have run 3 times, got %d run(s) (tally=%q)", len(data), data)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

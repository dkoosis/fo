package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/state"
)

func seedRunLog(t *testing.T, entries ...state.RunLogEntry) {
	t.Helper()
	t.Setenv("FO_STATE_DIR", t.TempDir())
	var rl *state.RunLog
	for _, e := range entries {
		rl = state.AppendRunLog(rl, e)
	}
	if err := state.SaveRunLog(state.RunLogPath(), rl); err != nil {
		t.Fatalf("seed run log: %v", err)
	}
}

func TestRunTrend_ChartsRisingRule(t *testing.T) {
	seedRunLog(t,
		state.RunLogEntry{RuleCounts: map[string]int{"SA1000": 1}},
		state.RunLogEntry{RuleCounts: map[string]int{"SA1000": 2}},
		state.RunLogEntry{RuleCounts: map[string]int{"SA1000": 4}},
	)
	var out, errBuf bytes.Buffer
	if code := runTrend([]string{"SA1000"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{"SA1000", "worsening +3", "runs 3", "peak 4"} {
		if !strings.Contains(got, want) {
			t.Errorf("trend output missing %q\n%s", want, got)
		}
	}
}

func TestRunTrend_UnknownRule(t *testing.T) {
	seedRunLog(t, state.RunLogEntry{RuleCounts: map[string]int{"SA1000": 1}})
	var out, errBuf bytes.Buffer
	if code := runTrend([]string{"NOPE"}, &out, &errBuf); code != 2 {
		t.Errorf("unknown rule: want exit 2, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "never appeared") {
		t.Errorf("want 'never appeared', got %q", errBuf.String())
	}
}

func TestRunTrend_NoHistory(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir())
	var out, errBuf bytes.Buffer
	if code := runTrend([]string{"SA1000"}, &out, &errBuf); code != 2 {
		t.Errorf("no history: want exit 2, got %d", code)
	}
}

func TestRunTrend_MissingArg(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := runTrend(nil, &out, &errBuf); code != 2 {
		t.Errorf("missing rule: want exit 2, got %d", code)
	}
}

func TestRunReplay_FiltersBySince(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	seedRunLog(t,
		state.RunLogEntry{At: now.Add(-2 * time.Hour), Tool: "old", Errors: 1},
		state.RunLogEntry{At: now, Tool: "new", Errors: 2},
	)
	var out, errBuf bytes.Buffer
	if code := runReplay([]string{"--since=1h"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "new") {
		t.Errorf("recent run should show: %s", got)
	}
	if strings.Contains(got, "old") {
		t.Errorf("run outside window should be filtered: %s", got)
	}
}

func TestRunReplay_AllWhenNoSince(t *testing.T) {
	seedRunLog(t,
		state.RunLogEntry{At: time.Now().Add(-100 * time.Hour), Tool: "old"},
		state.RunLogEntry{At: time.Now(), Tool: "new"},
	)
	var out, errBuf bytes.Buffer
	if code := runReplay(nil, &out, &errBuf); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if n := strings.Count(out.String(), "\n"); n != 2 {
		t.Errorf("want 2 runs listed, got %d lines", n)
	}
}

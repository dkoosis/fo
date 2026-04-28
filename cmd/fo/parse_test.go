package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestParseToReport_TolerantTestJSONPrelude verifies that go test -json input
// preceded by a non-JSON banner/prelude is still parsed (rather than rejected
// as unrecognized). Regression test for fo-fnw.
func TestParseToReport_TolerantTestJSONPrelude(t *testing.T) {
	prelude := "Running tests in CI...\n"
	events := strings.Join([]string{
		`{"Time":"2026-04-27T12:00:00Z","Action":"run","Package":"foo","Test":"TestA"}`,
		`{"Time":"2026-04-27T12:00:01Z","Action":"pass","Package":"foo","Test":"TestA","Elapsed":0.01}`,
		`{"Time":"2026-04-27T12:00:01Z","Action":"pass","Package":"foo","Elapsed":0.01}`,
	}, "\n") + "\n"
	input := []byte(prelude + events)

	var stderr bytes.Buffer
	r, err := parseToReport(input, &stderr)
	if err != nil {
		t.Fatalf("parseToReport: %v", err)
	}
	if r == nil {
		t.Fatal("parseToReport returned nil report")
	}
	if want := "fo: warning"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr should warn about malformed prelude lines; got: %q", stderr.String())
	}
}

// TestParseToReport_GarbageStillRejected verifies that input which is neither
// SARIF, multiplex, nor go test -json (even tolerantly) still returns an error.
func TestParseToReport_GarbageStillRejected(t *testing.T) {
	input := []byte("just some random text\nwith no JSON at all\n")
	var stderr bytes.Buffer
	if _, err := parseToReport(input, &stderr); err == nil {
		t.Fatal("expected error for unrecognized input, got nil")
	}
}

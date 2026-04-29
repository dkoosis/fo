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

// TestParseToReport_TruncatedTestJSONDiagnosed verifies that input which
// looks like go test -json but has only malformed/truncated JSON lines (so
// no events parse) returns a precise diagnostic mentioning the malformed
// count, rather than the generic 'unrecognized input'. Regression test for
// fo-6w5.
func TestParseToReport_TruncatedTestJSONDiagnosed(t *testing.T) {
	// Two truncated JSON lines — both fail to unmarshal, zero results aggregate.
	input := []byte(
		`{"Time":"2026-04-29T12:00:00Z","Action":"run","Package":"foo"` + "\n" +
			`{"Time":"2026-04-29T12:00:01Z","Action":"output` + "\n",
	)
	var stderr bytes.Buffer
	_, err := parseToReport(input, &stderr)
	if err == nil {
		t.Fatal("expected error for truncated JSON, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"go test -json", "failed to parse", "truncated"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "unrecognized input") {
		t.Errorf("truncated-stream input should not collapse to 'unrecognized input'; got: %q", msg)
	}
}

// TestParseToReport_UnknownMultiplexFormat verifies that a delimiter with the
// expected shape but an unsupported format value yields a precise error
// (section index, offending line, supported formats, and a hint pointing at
// 'fo wrap diag') rather than the generic 'unrecognized input'. Regression
// test for fo-y2o.
func TestParseToReport_UnknownMultiplexFormat(t *testing.T) {
	input := []byte("--- tool:build format:text ---\nbuild failed: foo.go:1: oops\n")
	var stderr bytes.Buffer
	_, err := parseToReport(input, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"section 1", `"text"`, "sarif", "testjson", "fo wrap diag"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should contain %q", msg, want)
		}
	}
}

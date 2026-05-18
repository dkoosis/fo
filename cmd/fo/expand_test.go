package main

import (
	"bytes"
	"strings"
	"testing"
)

// twoFailureClusterJSON crafts go test -json events for two failing tests
// in the same package, sharing an output frame so the clusterer groups them.
func twoFailureClusterJSON() string {
	const evt = `{"Time":"2026-05-17T00:00:00Z","Action":"%s","Package":"example.com/x","Test":"%s","Output":"%s"}`
	const frame = "    x_test.go:5: oops\\n"
	var b strings.Builder
	for _, name := range []string{"TestA", "TestB"} {
		b.WriteString("{\"Time\":\"2026-05-17T00:00:00Z\",\"Action\":\"run\",\"Package\":\"example.com/x\",\"Test\":\"" + name + "\"}\n")
		b.WriteString("{\"Time\":\"2026-05-17T00:00:00Z\",\"Action\":\"output\",\"Package\":\"example.com/x\",\"Test\":\"" + name + "\",\"Output\":\"" + frame + "\"}\n")
		b.WriteString("{\"Time\":\"2026-05-17T00:00:00Z\",\"Action\":\"output\",\"Package\":\"example.com/x\",\"Test\":\"" + name + "\",\"Output\":\"--- FAIL: " + name + " (0.00s)\\n\"}\n")
		b.WriteString("{\"Time\":\"2026-05-17T00:00:00Z\",\"Action\":\"fail\",\"Package\":\"example.com/x\",\"Test\":\"" + name + "\",\"Elapsed\":0}\n")
	}
	b.WriteString("{\"Time\":\"2026-05-17T00:00:00Z\",\"Action\":\"fail\",\"Package\":\"example.com/x\",\"Elapsed\":0}\n")
	_ = evt
	return b.String()
}

func TestExpand_UnknownIDWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"--format", "human", "--no-state", "--expand=cluster-deadbeef"},
		strings.NewReader(twoFailureClusterJSON()),
		&stdout, &stderr,
	)
	if code != 1 {
		t.Errorf("exit: got %d, want 1 (failures present). stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "fo: --expand=cluster-deadbeef not found") {
		t.Errorf("missing unknown-ID warning in stderr:\n%s", stderr.String())
	}
}

func TestExpand_LLMModeNoWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run(
		[]string{"--format", "llm", "--no-state", "--expand=cluster-deadbeef"},
		strings.NewReader(twoFailureClusterJSON()),
		&stdout, &stderr,
	)
	if strings.Contains(stderr.String(), "not found") {
		t.Errorf("LLM mode must not emit --expand warnings:\n%s", stderr.String())
	}
}

func TestExpand_AllShowsMembers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run(
		[]string{"--format", "human", "--no-state", "--expand=all"},
		strings.NewReader(twoFailureClusterJSON()),
		&stdout, &stderr,
	)
	// "all" suppresses unknown-ID warning.
	if strings.Contains(stderr.String(), "not found") {
		t.Errorf("--expand=all must not warn:\n%s", stderr.String())
	}
	// Both members should appear when expanded.
	if !strings.Contains(stdout.String(), "TestA") || !strings.Contains(stdout.String(), "TestB") {
		t.Errorf("--expand=all should show all members. stdout=\n%s", stdout.String())
	}
}

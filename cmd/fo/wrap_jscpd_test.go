package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWrapJscpd_EmptyDuplicates(t *testing.T) {
	input := `{"duplicates":[],"statistics":{}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"jscpd"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"schema":"fo-metrics/v1"`) && !strings.Contains(out, `"schema": "fo-metrics/v1"`) {
		t.Errorf("missing schema field in output:\n%s", out)
	}
	if !strings.Contains(out, `"status":"pass"`) && !strings.Contains(out, `"status": "pass"`) {
		t.Errorf("expected status pass:\n%s", out)
	}
}

func TestWrapJscpd_WithClones(t *testing.T) {
	input := `{"duplicates":[{
		"format":"go",
		"lines":22,
		"firstFile":{"name":"a.go","startLoc":{"line":1},"endLoc":{"line":22}},
		"secondFile":{"name":"b.go","startLoc":{"line":10},"endLoc":{"line":31}}
	}],"statistics":{}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"jscpd"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"warn"`) && !strings.Contains(out, `"status": "warn"`) {
		t.Errorf("expected status warn for clones:\n%s", out)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("expected clone files in details:\n%s", out)
	}
}

func TestWrapJscpd_InvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"jscpd"}, strings.NewReader("not json"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected error on stderr")
	}
}

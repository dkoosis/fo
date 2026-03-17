package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWrapArchlint_Clean(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":false,"ArchWarningsDeps":[],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"archlint"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"pass"`) && !strings.Contains(out, `"status": "pass"`) {
		t.Errorf("expected status pass:\n%s", out)
	}
	if !strings.Contains(out, "violations") {
		t.Errorf("expected violations metric in output:\n%s", out)
	}
}

func TestWrapArchlint_WithViolations(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"search","FileRelativePath":"pkg/search/search.go","ResolvedImportName":"embedder"}
	],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[]}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"archlint"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"fail"`) && !strings.Contains(out, `"status": "fail"`) {
		t.Errorf("expected status fail:\n%s", out)
	}
	if !strings.Contains(out, "search") || !strings.Contains(out, "embedder") {
		t.Errorf("expected violation details in output:\n%s", out)
	}
}

func TestWrapArchlint_InvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"archlint"}, strings.NewReader("bad"), &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

package view

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderStatus_human(t *testing.T) {
	rows := []StatusRow{
		{State: "ok", Label: "env-loaded"},
		{State: "fail", Label: "dolt-installed", Note: "not on PATH"},
		{State: "warn", Label: "snipe-fresh", Value: "2h-old"},
	}
	var buf bytes.Buffer
	if err := RenderStatusHuman(&buf, "doctor", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"doctor", "env-loaded", "dolt-installed", "not on PATH", "2h-old"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderStatus_llm(t *testing.T) {
	rows := []StatusRow{{State: "ok", Label: "a"}, {State: "fail", Label: "b"}}
	var buf bytes.Buffer
	if err := RenderStatusLLM(&buf, "tool", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "ok   a") || !strings.Contains(got, "fail b") {
		t.Errorf("llm output unexpected:\n%s", got)
	}
}

package view

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderStatus_human(t *testing.T) {
	rows := []StatusRow{
		{State: stateOK, Label: "env-loaded"},
		{State: stateFail, Label: "dolt-installed", Note: "not on PATH"},
		{State: stateWarn, Label: "snipe-fresh", Value: "2h-old"},
	}
	var buf bytes.Buffer
	if err := RenderStatusHuman(&buf, "doctor", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	// Computed summary line: 1 ok / 1 fail / 1 warn / 0 skip.
	if !strings.Contains(out, "1 ok · 1 fail · 1 warn · 0 skip") {
		t.Errorf("missing or wrong summary counts in output:\n%s", out)
	}
	// State column must pair each state with its row — a renderer that drops
	// the state glyph would still pass a bare label-substring check.
	for _, want := range []string{
		"doctor",
		"ok   env-loaded",
		"fail dolt-installed",
		"not on PATH",
		"warn snipe-fresh",
		"2h-old",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderStatus_llm(t *testing.T) {
	rows := []StatusRow{{State: stateOK, Label: "a"}, {State: stateFail, Label: "b"}}
	var buf bytes.Buffer
	if err := RenderStatusLLM(&buf, "tool", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "ok   a") || !strings.Contains(got, "fail b") {
		t.Errorf("llm output unexpected:\n%s", got)
	}
}

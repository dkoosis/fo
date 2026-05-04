package view

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderMetrics_human(t *testing.T) {
	rows := []MetricRow{
		{Key: "pkg/x", Value: 87.3, Unit: "%", Delta: 7.3},
		{Key: "pkg/y", Value: 100, Unit: "%", Delta: 0},
	}
	var buf bytes.Buffer
	if err := RenderMetricsHuman(&buf, "cover", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"cover", "pkg/x", "87.3", "%", "+7.3"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderMetrics_llm(t *testing.T) {
	rows := []MetricRow{{Key: "k", Value: 1.5, Unit: "s"}}
	var buf bytes.Buffer
	if err := RenderMetricsLLM(&buf, "tool", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "k 1.5 s") {
		t.Errorf("got: %q", got)
	}
}

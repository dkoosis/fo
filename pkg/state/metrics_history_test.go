package state

import (
	"path/filepath"
	"testing"
)

func TestMetricsHistory_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	curr := []MetricSample{
		{Tool: "cover", Key: "pkg/x", Value: 87.3, Unit: "%"},
		{Tool: "cover", Key: "pkg/y", Value: 100, Unit: "%"},
	}
	if err := SaveMetrics(path, curr); err != nil {
		t.Fatalf("save: %v", err)
	}
	prev, err := LoadMetrics(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(prev) != 2 || prev[0].Value != 87.3 {
		t.Errorf("got %+v", prev)
	}
}

func TestMetricsHistory_diff(t *testing.T) {
	prev := []MetricSample{{Tool: "cover", Key: "pkg/x", Value: 80}}
	curr := []MetricSample{{Tool: "cover", Key: "pkg/x", Value: 87.3}}
	d := DiffMetrics(prev, curr)
	if len(d) != 1 || !floatEq(d[0].Delta, 7.3) || d[0].New {
		t.Errorf("diff = %+v", d)
	}
}

func TestMetricsHistory_newRow(t *testing.T) {
	curr := []MetricSample{{Tool: "cover", Key: "pkg/new", Value: 42}}
	d := DiffMetrics(nil, curr)
	if len(d) != 1 || !d[0].New || d[0].Delta != 0 {
		t.Errorf("expected New=true, Delta=0; got %+v", d)
	}
}

func TestMetricsHistory_keyOnlyFallback(t *testing.T) {
	prev := []MetricSample{{Tool: "cover", Key: "pkg/x", Value: 80}}
	curr := []MetricSample{{Tool: "", Key: "pkg/x", Value: 90}}
	d := DiffMetrics(prev, curr)
	if len(d) != 1 || d[0].Delta != 10 || d[0].New {
		t.Errorf("expected key-only match Delta=10, got %+v", d)
	}
}

func floatEq(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

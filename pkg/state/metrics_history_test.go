package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeLegacyFlatMetrics(t *testing.T, path string, samples []MetricSample) error {
	t.Helper()
	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

const (
	coverName = "cover"
	pkgXKey   = "pkg/x"
)

func TestMetricsHistory_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	curr := []MetricSample{
		{Tool: coverName, Key: pkgXKey, Value: 87.3, Unit: "%"},
		{Tool: coverName, Key: "pkg/y", Value: 100, Unit: "%"},
	}
	if err := AppendMetrics(path, curr); err != nil {
		t.Fatalf("append: %v", err)
	}
	prev, err := LoadMetrics(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(prev) != 2 || prev[0].Value != 87.3 {
		t.Errorf("got %+v", prev)
	}
}

// TestAppendMetrics_AccumulatesAndTrims verifies multi-run history retention
// (regression for #258 / fo-2nj — file used to overwrite, never append).
func TestAppendMetrics_AccumulatesAndTrims(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	for i := range MaxMetricsHistory + 5 {
		samples := []MetricSample{{Tool: coverName, Key: pkgXKey, Value: float64(i)}}
		if err := AppendMetrics(path, samples); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	hist, err := LoadMetricsHistory(path)
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if len(hist.Runs) != MaxMetricsHistory {
		t.Fatalf("runs = %d, want %d", len(hist.Runs), MaxMetricsHistory)
	}
	// Newest run is at index 0; should hold the last value written.
	got := hist.Runs[0].Samples[0].Value
	want := float64(MaxMetricsHistory + 4)
	if got != want {
		t.Fatalf("newest value = %v, want %v", got, want)
	}
	// Oldest retained run should be N-MaxMetricsHistory+1 (i.e. trimmed off the older ones).
	gotOld := hist.Runs[len(hist.Runs)-1].Samples[0].Value
	wantOld := float64(5) // 5 oldest were trimmed (0..4)
	if gotOld != wantOld {
		t.Fatalf("oldest value = %v, want %v", gotOld, wantOld)
	}
}

// TestLoadMetricsHistory_LegacyFlatFile verifies back-compat: pre-existing
// flat []MetricSample files load as a single-run envelope so users don't
// lose their last sample on upgrade.
func TestLoadMetricsHistory_LegacyFlatFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	if err := writeLegacyFlatMetrics(t, path, []MetricSample{
		{Tool: coverName, Key: pkgXKey, Value: 42},
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	hist, err := LoadMetricsHistory(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(hist.Runs) != 1 || len(hist.Runs[0].Samples) != 1 || hist.Runs[0].Samples[0].Value != 42 {
		t.Fatalf("legacy load mismatch: %+v", hist)
	}
}

func TestMetricsHistory_diff(t *testing.T) {
	prev := []MetricSample{{Tool: coverName, Key: pkgXKey, Value: 80}}
	curr := []MetricSample{{Tool: coverName, Key: pkgXKey, Value: 87.3}}
	d := DiffMetrics(prev, curr)
	if len(d) != 1 || !floatEq(d[0].Delta, 7.3) || d[0].New {
		t.Errorf("diff = %+v", d)
	}
}

func TestMetricsHistory_newRow(t *testing.T) {
	curr := []MetricSample{{Tool: coverName, Key: "pkg/new", Value: 42}}
	d := DiffMetrics(nil, curr)
	if len(d) != 1 || !d[0].New || d[0].Delta != 0 {
		t.Errorf("expected New=true, Delta=0; got %+v", d)
	}
}

func TestMetricsHistory_keyOnlyFallback(t *testing.T) {
	prev := []MetricSample{{Tool: coverName, Key: pkgXKey, Value: 80}}
	curr := []MetricSample{{Tool: "", Key: pkgXKey, Value: 90}}
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

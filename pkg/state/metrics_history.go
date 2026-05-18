package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// MaxMetricsHistory bounds the number of prior runs retained in
// metrics-history.json. Sized for trend windows / sparklines, not just
// the t-1 vs t delta arrow shown by DiffMetrics.
const MaxMetricsHistory = 30

// MetricsSchemaVersion identifies the on-disk envelope format. Bump when
// MetricsFile/MetricsRun shape changes incompatibly.
const MetricsSchemaVersion = 1

type MetricSample struct {
	Tool  string  `json:"tool,omitempty"`
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type MetricDelta struct {
	Sample MetricSample `json:"sample"`
	Prior  float64      `json:"prior"`
	Delta  float64      `json:"delta"`
	New    bool         `json:"new"` // no prior sample matched
}

// MetricsRun is one captured set of samples at a point in time.
type MetricsRun struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Samples     []MetricSample `json:"samples"`
}

// MetricsFile is the versioned envelope written to metrics-history.json.
// Runs[0] is the newest; Runs[len-1] the oldest. Mirrors state.File.Runs
// ordering so consumers can treat the two histories the same way.
type MetricsFile struct {
	Version int          `json:"version"`
	Runs    []MetricsRun `json:"runs"`
}

// LoadMetricsHistory reads the versioned envelope from path. A missing
// file returns an empty file with no error. A pre-envelope flat
// []MetricSample is read as a single-run envelope so users keep their
// last sample after the format change.
func LoadMetricsHistory(path string) (*MetricsFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &MetricsFile{Version: MetricsSchemaVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: read %s: %w", path, err)
	}
	var envelope MetricsFile
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version > 0 {
		return &envelope, nil
	}
	// Legacy: flat []MetricSample.
	var legacy []MetricSample
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("metrics: unmarshal: %w", err)
	}
	return &MetricsFile{
		Version: MetricsSchemaVersion,
		Runs:    []MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: legacy}},
	}, nil
}

// LoadMetrics returns the newest run's samples, or nil if no history
// exists. Preserved for callers (DiffMetrics consumers) that only care
// about the latest snapshot.
func LoadMetrics(path string) ([]MetricSample, error) {
	hist, err := LoadMetricsHistory(path)
	if err != nil {
		return nil, err
	}
	if len(hist.Runs) == 0 {
		return nil, nil
	}
	return hist.Runs[0].Samples, nil
}

// AppendMetrics loads existing history, prepends a new run with the
// current samples, trims to MaxMetricsHistory, and writes the envelope
// back. Replaces the prior overwrite-only SaveMetrics (#258).
func AppendMetrics(path string, samples []MetricSample) error {
	hist, err := LoadMetricsHistory(path)
	if err != nil {
		return err
	}
	hist.Version = MetricsSchemaVersion
	hist.Runs = append([]MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: samples}}, hist.Runs...)
	if len(hist.Runs) > MaxMetricsHistory {
		hist.Runs = hist.Runs[:MaxMetricsHistory]
	}
	data, err := json.MarshalIndent(hist, "", "  ")
	if err != nil {
		return fmt.Errorf("metrics: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("metrics: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".metrics-history.*.tmp")
	if err != nil {
		return fmt.Errorf("metrics: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("metrics: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("metrics: fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("metrics: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("metrics: rename: %w", err)
	}
	return nil
}

// DiffMetrics matches each curr sample to prev by tool+key, falling back
// to key alone when curr has no tool tag (so `--as metrics` injection
// still gets real deltas, not spurious New=true rows).
func DiffMetrics(prev, curr []MetricSample) []MetricDelta {
	priorTK := make(map[string]float64, len(prev))
	priorK := make(map[string]float64, len(prev))
	for _, s := range prev {
		priorTK[s.Tool+"\x00"+s.Key] = s.Value
		priorK[s.Key] = s.Value
	}
	out := make([]MetricDelta, 0, len(curr))
	for _, s := range curr {
		if p, ok := priorTK[s.Tool+"\x00"+s.Key]; ok {
			out = append(out, MetricDelta{Sample: s, Prior: p, Delta: s.Value - p})
			continue
		}
		if p, ok := priorK[s.Key]; ok && s.Tool == "" {
			out = append(out, MetricDelta{Sample: s, Prior: p, Delta: s.Value - p})
			continue
		}
		out = append(out, MetricDelta{Sample: s, Prior: 0, Delta: 0, New: true})
	}
	return out
}

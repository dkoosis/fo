package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

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
	New    bool         `json:"new,omitempty"` // no prior sample matched
}

func SaveMetrics(path string, samples []MetricSample) error {
	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		return fmt.Errorf("metrics: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("metrics: write %s: %w", path, err)
	}
	return nil
}

func LoadMetrics(path string) ([]MetricSample, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: read %s: %w", path, err)
	}
	var out []MetricSample
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("metrics: unmarshal: %w", err)
	}
	return out, nil
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

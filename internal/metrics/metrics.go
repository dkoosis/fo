package metrics

import "encoding/json"

// Report represents a generic metrics report (eval, benchmarks, etc.).
type Report struct {
	Scope       string       `json:"scope"`
	Columns     []string     `json:"columns"`
	Rows        []Row        `json:"rows"`
	Regressions []Regression `json:"regressions"`
}

// Row is a single named row of metric values.
type Row struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
	N      int       `json:"n,omitempty"`
}

// Regression records a metric that got worse between runs.
type Regression struct {
	Group  string  `json:"group"`
	Metric string  `json:"metric"`
	From   float64 `json:"from"`
	To     float64 `json:"to"`
}

// Parse decodes metrics JSON into a Report.
func Parse(data []byte) (*Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

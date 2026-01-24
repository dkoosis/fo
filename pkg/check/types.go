// Package check provides lintkit-check format parsing and rendering.
// The lintkit-check format is a structured output format for lint tools
// that provides summary, metrics, items, and trend data.
package check

// Report represents a lintkit-check document.
type Report struct {
	Schema  string   `json:"$schema"`
	Tool    string   `json:"tool"`
	Status  string   `json:"status"` // "pass", "warn", "fail"
	Summary string   `json:"summary"`
	Metrics []Metric `json:"metrics,omitempty"`
	Items   []Item   `json:"items,omitempty"`
	Trend   []int    `json:"trend,omitempty"`
}

// Metric represents a single metric measurement.
type Metric struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold,omitempty"`
	Unit      string  `json:"unit,omitempty"`
}

// Item represents a single finding/issue.
type Item struct {
	Severity string `json:"severity"` // "error", "warning", "info"
	Label    string `json:"label"`
	Value    string `json:"value,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
}

// StatusPass indicates the check passed with no issues.
const StatusPass = "pass"

// StatusWarn indicates the check completed with warnings.
const StatusWarn = "warn"

// StatusFail indicates the check failed.
const StatusFail = "fail"

// SeverityError indicates an error-level item.
const SeverityError = "error"

// SeverityWarning indicates a warning-level item.
const SeverityWarning = "warning"

// SeverityInfo indicates an informational item.
const SeverityInfo = "info"

package fometrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Document represents a fo-metrics/v1 JSON object.
type Document struct {
	Schema  string   `json:"schema"`
	Tool    string   `json:"tool"`
	Status  string   `json:"status"`
	Metrics []Metric `json:"metrics"`
	Summary string   `json:"summary,omitempty"`
	Details []Detail `json:"details,omitempty"`
}

// Metric is a single named metric with optional threshold and direction.
type Metric struct {
	Name      string   `json:"name"`
	Value     float64  `json:"value"`
	Threshold *float64 `json:"threshold,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	Direction string   `json:"direction,omitempty"`
}

// Detail is an itemized finding within a metrics report.
type Detail struct {
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity,omitempty"`
	Category string `json:"category,omitempty"`
}

// Parse decodes and validates a fo-metrics JSON document.
// Accepts fo-metrics/v1 and v1.x; rejects v2+ and missing/malformed schemas.
func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	if err := validate(&doc); err != nil {
		return nil, err
	}

	applyDefaults(&doc)
	return &doc, nil
}

func validate(doc *Document) error {
	switch {
	case doc.Schema == "":
		return fmt.Errorf("missing required field: schema")
	case doc.Schema == "fo-metrics/v1":
		// exact match, ok
	case strings.HasPrefix(doc.Schema, "fo-metrics/v1."):
		// minor version, ok
	default:
		return fmt.Errorf("unsupported schema: %s", doc.Schema)
	}

	if doc.Tool == "" {
		return fmt.Errorf("missing required field: tool")
	}

	switch doc.Status {
	case "pass", "fail", "warn":
		// valid
	case "":
		return fmt.Errorf("missing required field: status")
	default:
		return fmt.Errorf("invalid status: %q (expected pass, fail, or warn)", doc.Status)
	}

	return nil
}

func applyDefaults(doc *Document) {
	for i := range doc.Metrics {
		if doc.Metrics[i].Direction == "" {
			doc.Metrics[i].Direction = "higher_is_better"
		}
	}
}

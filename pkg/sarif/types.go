// Package sarif provides SARIF (Static Analysis Results Interchange Format) parsing and rendering.
package sarif

import "encoding/json"

// Document represents a SARIF 2.1.0 document.
// See: https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html
type Document struct {
	Version string `json:"version"`
	Schema  string `json:"$schema,omitempty"`
	Runs    []Run  `json:"runs"`
}

// Run represents a single analysis run.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool identifies the analysis tool that produced the results.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver describes the tool's identity.
type Driver struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Result represents a single issue found by the tool.
type Result struct {
	RuleID    string           `json:"ruleId"`
	Level     string           `json:"level"` // "error", "warning", "note", "none"
	Message   Message          `json:"message"`
	Locations []Location       `json:"locations,omitempty"`
	Related   []Location       `json:"relatedLocations,omitempty"`
	Props     json.RawMessage  `json:"properties,omitempty"`
}

// Message contains the issue description.
type Message struct {
	Text string `json:"text"`
}

// Location identifies where the issue was found.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation pinpoints the file and region.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region,omitempty"`
}

// ArtifactLocation identifies the file.
type ArtifactLocation struct {
	URI   string `json:"uri"`
	Index int    `json:"index,omitempty"`
}

// Region identifies the specific location within the file.
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

// FilePath returns the normalized file path from a result's primary location.
// Strips file:// prefix and normalizes separators.
func (r *Result) FilePath() string {
	if len(r.Locations) == 0 {
		return ""
	}
	uri := r.Locations[0].PhysicalLocation.ArtifactLocation.URI
	return NormalizePath(uri)
}

// Line returns the start line from a result's primary location.
func (r *Result) Line() int {
	if len(r.Locations) == 0 {
		return 0
	}
	return r.Locations[0].PhysicalLocation.Region.StartLine
}

// Col returns the start column from a result's primary location.
func (r *Result) Col() int {
	if len(r.Locations) == 0 {
		return 0
	}
	return r.Locations[0].PhysicalLocation.Region.StartColumn
}

// NormalizePath strips file:// prefix and cleans a SARIF URI to a relative path.
func NormalizePath(uri string) string {
	if len(uri) > 7 && uri[:7] == "file://" {
		uri = uri[7:]
	}
	return uri
}

// Package sarif provides SARIF (Static Analysis Results Interchange Format) parsing and rendering.
package sarif

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
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"` // "error", "warning", "note", "none"
	Message   Message    `json:"message"`
	Locations []Location `json:"locations,omitempty"`
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

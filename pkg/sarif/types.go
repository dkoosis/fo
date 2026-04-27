// Package sarif provides SARIF (Static Analysis Results Interchange Format) parsing and rendering.
package sarif

// SARIF result levels per the 2.1.0 spec.
const (
	LevelError   = "error"
	LevelWarning = "warning"
	LevelNote    = "note"
	LevelNone    = "none"
)

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
	Fixes     []Fix      `json:"fixes,omitempty"`
}

// Fix proposes an action to resolve a result. fo uses Description.Text to
// carry a shell command (or grep-ready hint) that the user can run to fix
// or investigate the finding — a fo-specific, spec-compatible use of the
// SARIF 2.1.0 "fixes" field.
type Fix struct {
	Description Message `json:"description"`
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
	Region           Region           `json:"region,omitzero"`
}

// ArtifactLocation identifies the file.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// Region identifies the specific location within the file.
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
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

// FixCommand returns the first fix description text, or "" if no fix is
// attached. fo wrappers put a shell command here during SARIF construction.
func (r *Result) FixCommand() string {
	if len(r.Fixes) == 0 {
		return ""
	}
	return r.Fixes[0].Description.Text
}

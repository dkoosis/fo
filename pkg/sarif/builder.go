package sarif

import (
	"encoding/json"
	"fmt"
	"io"
)

// Builder constructs valid SARIF 2.1.0 documents.
// Designed for fo wrap and as an importable library.
type Builder struct {
	doc *Document
	err error
}

// NewBuilder creates a SARIF builder for the given tool.
func NewBuilder(toolName, toolVersion string) *Builder {
	return &Builder{
		doc: &Document{
			Version: "2.1.0",
			Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
			Runs: []Run{{
				Tool: Tool{
					Driver: Driver{
						Name:    toolName,
						Version: toolVersion,
					},
				},
			}},
		},
	}
}

// validLevel reports whether level is a valid SARIF result level.
func validLevel(level string) bool {
	switch level {
	case "error", "warning", "note", "none":
		return true
	}
	return false
}

// AddResult adds a diagnostic result to the current run.
func (b *Builder) AddResult(ruleID, level, message, file string, line, col int) *Builder {
	if b.err != nil {
		return b
	}
	if !validLevel(level) {
		b.err = fmt.Errorf("sarif: invalid level %q; must be error, warning, note, or none", level)
		return b
	}
	r := Result{
		RuleID:  ruleID,
		Level:   level,
		Message: Message{Text: message},
	}
	if file != "" {
		r.Locations = []Location{{
			PhysicalLocation: PhysicalLocation{
				ArtifactLocation: ArtifactLocation{URI: file},
				Region: Region{
					StartLine:   line,
					StartColumn: col,
				},
			},
		}}
	}
	b.doc.Runs[0].Results = append(b.doc.Runs[0].Results, r)
	return b
}

// Document returns the constructed SARIF document without validation.
// Use WriteTo for production output — it validates driver name and levels.
// This method is the "I know what I'm doing" escape hatch for tests and inspection.
func (b *Builder) Document() *Document {
	return b.doc
}

// WriteTo writes the SARIF document as JSON to w.
// Returns an error if the driver name is empty or any result has an invalid level.
func (b *Builder) WriteTo(w io.Writer) (int64, error) {
	if b.doc.Runs[0].Tool.Driver.Name == "" {
		return 0, fmt.Errorf("sarif: driver name must not be empty")
	}
	if b.err != nil {
		return 0, b.err
	}
	data, err := json.MarshalIndent(b.doc, "", "  ")
	if err != nil {
		return 0, err
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	return int64(n), err
}

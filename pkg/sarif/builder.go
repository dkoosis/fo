package sarif

import (
	"encoding/json"
	"io"
)

// Builder constructs valid SARIF 2.1.0 documents.
// Designed for fo wrap and as an importable library.
type Builder struct {
	doc *Document
	run *Run
}

// NewBuilder creates a SARIF builder for the given tool.
func NewBuilder(toolName, toolVersion string) *Builder {
	run := Run{
		Tool: Tool{
			Driver: Driver{
				Name:    toolName,
				Version: toolVersion,
			},
		},
	}
	return &Builder{
		doc: &Document{
			Version: "2.1.0",
			Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
			Runs:    []Run{run},
		},
		run: &run,
	}
}

// AddResult adds a diagnostic result to the current run.
func (b *Builder) AddResult(ruleID, level, message, file string, line, col int) *Builder {
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
	b.run.Results = append(b.run.Results, r)
	// Keep doc in sync
	b.doc.Runs[0] = *b.run
	return b
}

// Document returns the constructed SARIF document.
func (b *Builder) Document() *Document {
	return b.doc
}

// WriteTo writes the SARIF document as JSON to w.
func (b *Builder) WriteTo(w io.Writer) (int64, error) {
	data, err := json.MarshalIndent(b.doc, "", "  ")
	if err != nil {
		return 0, err
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	return int64(n), err
}

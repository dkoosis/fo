package report

import _ "embed"

//go:embed report.schema.json
var schemaJSON string

// Schema returns the JSON Schema (draft 2020-12) describing the Report shape
// emitted by `fo --format json`. Consumers can use it to generate types or
// validate output. Keep in sync with the Go types in this file.
func Schema() string {
	return schemaJSON
}

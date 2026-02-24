package render

import (
	"encoding/json"

	"github.com/dkoosis/fo/pkg/pattern"
)

// JSON renders patterns as structured JSON for automation.
type JSON struct{}

// NewJSON creates a JSON renderer.
func NewJSON() *JSON {
	return &JSON{}
}

// jsonOutput is the top-level JSON structure.
type jsonOutput struct {
	Version  string        `json:"version"`
	Patterns []jsonPattern `json:"patterns"`
}

type jsonPattern struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Render formats all patterns as JSON.
func (j *JSON) Render(patterns []pattern.Pattern) string {
	out := jsonOutput{
		Version:  "2.0",
		Patterns: make([]jsonPattern, 0, len(patterns)),
	}

	for _, p := range patterns {
		out.Patterns = append(out.Patterns, jsonPattern{
			Type: string(p.Type()),
			Data: p,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errJSON)
	}
	return string(data) + "\n"
}

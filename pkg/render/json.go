package render

import (
	"encoding/json"

	"github.com/dkoosis/fo/pkg/pattern"
)

// JSON renders patterns as structured JSON for automation.
type JSON struct {
	meta RunMeta
}

// NewJSON creates a JSON renderer.
func NewJSON() *JSON {
	return &JSON{}
}

// WithMeta attaches run envelope metadata for rendering.
func (j *JSON) WithMeta(m RunMeta) *JSON {
	j.meta = m
	return j
}

// jsonOutput is the top-level JSON structure.
type jsonOutput struct {
	Version     string        `json:"version"`
	DataHash    string        `json:"data_hash,omitempty"`
	GeneratedAt string        `json:"generated_at,omitempty"`
	Patterns    []jsonPattern `json:"patterns"`
}

type jsonPattern struct {
	Type string      `json:"type"`
	Data any `json:"data"`
}

// Render formats all patterns as JSON.
func (j *JSON) Render(patterns []pattern.Pattern) string {
	out := jsonOutput{
		Version:     "2.0",
		DataHash:    j.meta.DataHash,
		GeneratedAt: j.meta.GeneratedAt,
		Patterns:    make([]jsonPattern, 0, len(patterns)),
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

// Package render provides output renderers for fo's visualization patterns.
package render

import "github.com/dkoosis/fo/pkg/pattern"

// Renderer converts patterns to formatted output.
type Renderer interface {
	Render(patterns []pattern.Pattern) string
}

// RunMeta carries envelope metadata about a single fo run.
// Hash is computed once over the input bytes; GeneratedAt is set when the run starts.
// Rendered by llm and json renderers; ignored by the human renderer.
type RunMeta struct {
	DataHash    string `json:"data_hash"`
	GeneratedAt string `json:"generated_at"`
}

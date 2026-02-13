// Package render provides output renderers for fo's visualization patterns.
package render

import "github.com/dkoosis/fo/pkg/pattern"

// Renderer converts patterns to formatted output.
type Renderer interface {
	Render(patterns []pattern.Pattern) string
}

package archlint

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// Adapter parses and renders go-arch-lint JSON output.
type Adapter struct {
	theme *design.Config
}

// NewAdapter creates a new adapter with the given theme.
func NewAdapter(theme *design.Config) *Adapter {
	return &Adapter{theme: theme}
}

// Parse reads go-arch-lint JSON from a reader.
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	var result Result
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseBytes parses go-arch-lint JSON from bytes.
func (a *Adapter) ParseBytes(data []byte) (*Result, error) {
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Render renders a go-arch-lint result to a string.
func (a *Adapter) Render(result *Result) string {
	mapper := NewMapper()
	patterns := mapper.MapToPatterns(result)

	var sb strings.Builder
	for i, p := range patterns {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(p.Render(a.theme))
	}

	return sb.String()
}

// RenderReader reads and renders go-arch-lint JSON from a reader.
func (a *Adapter) RenderReader(r io.Reader) (string, error) {
	result, err := a.Parse(r)
	if err != nil {
		return "", err
	}
	return a.Render(result), nil
}

// RenderFile reads and renders go-arch-lint JSON from a file.
func (a *Adapter) RenderFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return a.RenderReader(f)
}

// QuickRender renders go-arch-lint JSON with default theme.
func QuickRender(data []byte) (string, error) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseBytes(data)
	if err != nil {
		return "", err
	}
	return adapter.Render(result), nil
}

// IsArchLintJSON detects if the data looks like go-arch-lint JSON output.
func IsArchLintJSON(data []byte) bool {
	// Quick heuristic check
	if len(data) < 20 {
		return false
	}

	// Look for go-arch-lint signature
	s := string(data[:min(500, len(data))])
	return strings.Contains(s, `"Type"`) &&
		strings.Contains(s, `"Payload"`) &&
		(strings.Contains(s, `"ArchHasWarnings"`) || strings.Contains(s, `"ArchWarningsDeps"`))
}

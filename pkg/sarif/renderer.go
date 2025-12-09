package sarif

import (
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// Renderer renders SARIF documents using fo's pattern system.
type Renderer struct {
	config  RendererConfig
	foTheme *design.Config
}

// NewRenderer creates a renderer with the given configuration.
func NewRenderer(config RendererConfig, foTheme *design.Config) *Renderer {
	return &Renderer{
		config:  config,
		foTheme: foTheme,
	}
}

// Render renders a SARIF document to a string.
func (r *Renderer) Render(doc *Document) string {
	if doc == nil || len(doc.Runs) == 0 {
		return ""
	}

	// Get tool name from first run
	toolName := ""
	if len(doc.Runs) > 0 {
		toolName = doc.Runs[0].Tool.Driver.Name
	}

	// Get tool-specific config or use defaults
	toolConfig, ok := r.config.Tools[toolName]
	if !ok {
		toolConfig = r.config.Defaults
	}

	// Map SARIF to patterns
	mapper := NewMapper(toolConfig)
	patterns := mapper.MapToPatterns(doc)

	// Render each pattern
	var sb strings.Builder
	for i, p := range patterns {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(p.Render(r.foTheme))
	}

	return sb.String()
}

// RenderFile reads and renders a SARIF file.
func (r *Renderer) RenderFile(path string) (string, error) {
	doc, err := ReadFile(path)
	if err != nil {
		return "", err
	}
	return r.Render(doc), nil
}

// QuickRender renders a SARIF file with default settings.
func QuickRender(path string) (string, error) {
	doc, err := ReadFile(path)
	if err != nil {
		return "", err
	}

	config := DefaultRendererConfig()
	theme := design.DefaultConfig()

	renderer := NewRenderer(config, theme)
	return renderer.Render(doc), nil
}

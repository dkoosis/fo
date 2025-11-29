// Package fo provides a processor for handling command output.
package fo

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/dkoosis/fo/pkg/adapter"
	"github.com/dkoosis/fo/pkg/design"
)

// Processor handles processing of command output, including classification
// and adapter-based parsing.
type Processor struct {
	patternMatcher  *design.PatternMatcher
	adapterRegistry *adapter.Registry
	maxLineLength   int
	debug           bool
}

// NewProcessor creates a new Processor with the given configuration.
func NewProcessor(
	patternMatcher *design.PatternMatcher,
	adapterRegistry *adapter.Registry,
	maxLineLength int,
	debug bool,
) *Processor {
	return &Processor{
		patternMatcher:  patternMatcher,
		adapterRegistry: adapterRegistry,
		maxLineLength:   maxLineLength,
		debug:           debug,
	}
}

// ProcessOutput processes buffered output, attempting adapter-based parsing first,
// then falling back to line-by-line classification.
func (p *Processor) ProcessOutput(
	task *design.Task,
	output []byte,
	command string,
	args []string,
) {
	// Extract first N lines for adapter detection
	const adapterDetectionLineCount = 15
	firstLines := extractFirstLines(string(output), adapterDetectionLineCount)

	// Try to detect a suitable adapter
	detectedAdapter := p.adapterRegistry.Detect(firstLines)

	if detectedAdapter != nil {
		// Parse with adapter
		pattern, parseErr := detectedAdapter.Parse(bytes.NewReader(output))
		if parseErr == nil && pattern != nil {
			// Render the pattern using the design config
			rendered := pattern.Render(task.Config)
			if rendered != "" {
				// Add the rendered pattern as output
				task.AddOutputLine(rendered, design.TypeDetail, design.LineContext{
					CognitiveLoad: design.LoadLow,
					Importance:    4,
					IsInternal:    false,
				})
			}
			return
		}
	}

	// Fall back to line-by-line classification
	p.processLineByLine(task, string(output), command, args)
}

// processLineByLine processes output with line-by-line classification.
func (p *Processor) processLineByLine(
	task *design.Task,
	output string,
	command string,
	args []string,
) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	buf := make([]byte, 0, bufio.MaxScanTokenSize)
	scanner.Buffer(buf, p.maxLineLength)

	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineType, lineContext := p.patternMatcher.ClassifyOutputLine(line, command, args)
		task.AddOutputLine(line, lineType, lineContext)
		lineCount++
	}
}

// extractFirstLines extracts the first N lines from the output for adapter detection.
func extractFirstLines(output string, count int) []string {
	lines := strings.Split(output, "\n")
	if len(lines) > count {
		lines = lines[:count]
	}
	return lines
}

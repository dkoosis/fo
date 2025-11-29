// Package fo provides a processor for handling command output.
package fo

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/adapter"
	"github.com/dkoosis/fo/pkg/design"
)

// LineCallback is called for each processed line during streaming.
// It receives the line content, its classified type, and context.
type LineCallback func(line string, lineType string, ctx design.LineContext)

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

// ProcessStream processes input from an io.Reader line-by-line, calling onLine
// for each classified line. This enables live rendering as output arrives.
//
// The processor buffers the first N lines for adapter detection. If an adapter
// matches, it processes buffered lines through the adapter, then streams the rest.
// Otherwise, it falls back to line-by-line classification.
func (p *Processor) ProcessStream(
	input io.Reader,
	command string,
	args []string,
	onLine LineCallback,
) error {
	const adapterDetectionLineCount = 15

	scanner := bufio.NewScanner(input)
	buf := make([]byte, 0, bufio.MaxScanTokenSize)
	scanner.Buffer(buf, p.maxLineLength)

	// Buffer first N lines for adapter detection
	var bufferedLines []string
	for scanner.Scan() && len(bufferedLines) < adapterDetectionLineCount {
		bufferedLines = append(bufferedLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Try adapter detection on buffered lines
	detectedAdapter := p.adapterRegistry.Detect(bufferedLines)

	if detectedAdapter != nil {
		// Collect remaining lines for adapter parsing
		for scanner.Scan() {
			bufferedLines = append(bufferedLines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return err
		}

		// Parse with adapter
		allContent := strings.Join(bufferedLines, "\n")
		pattern, parseErr := detectedAdapter.Parse(strings.NewReader(allContent))
		if parseErr == nil && pattern != nil {
			// For adapter output, emit as single rendered block
			// (adapters produce their own formatted output)
			onLine(pattern.Render(nil), design.TypeDetail, design.LineContext{
				CognitiveLoad: design.LoadLow,
				Importance:    4,
				IsInternal:    false,
			})
			return nil
		}
		// Adapter failed - fall through to line-by-line
	}

	// Line-by-line classification for buffered lines
	for _, line := range bufferedLines {
		lineType, lineContext := p.patternMatcher.ClassifyOutputLine(line, command, args)
		onLine(line, lineType, lineContext)
	}

	// Continue with remaining lines from scanner
	for scanner.Scan() {
		line := scanner.Text()
		lineType, lineContext := p.patternMatcher.ClassifyOutputLine(line, command, args)
		onLine(line, lineType, lineContext)
	}

	return scanner.Err()
}

// Package fo provides a processor for handling command output.
package fo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// LineCallback is called for each processed line during streaming.
// It receives the line content, its classified type, and context.
type LineCallback func(line string, lineType string, ctx design.LineContext)

// Processor handles processing of command output with line classification.
type Processor struct {
	patternMatcher *design.PatternMatcher
	maxLineLength  int
	debug          bool
}

// NewProcessor creates a new Processor with the given configuration.
func NewProcessor(
	patternMatcher *design.PatternMatcher,
	maxLineLength int,
	debug bool,
) *Processor {
	return &Processor{
		patternMatcher: patternMatcher,
		maxLineLength:  maxLineLength,
		debug:          debug,
	}
}

// ProcessOutput processes buffered output.
// SARIF format is detected and stored for specialized rendering.
// Other formats go through line-by-line classification.
func (p *Processor) ProcessOutput(
	task *design.Task,
	output []byte,
	command string,
	args []string,
) {
	// SARIF gets stored for specialized rendering - skip line-by-line
	if isSARIF(output) {
		// Extract just the SARIF JSON (tools may append text after it)
		task.IsSARIF = true
		task.SARIFData = extractSARIF(output)
		return
	}
	p.processLineByLine(task, string(output), command, args)
}

// isSARIF checks if data looks like a SARIF document.
// It handles tools like golangci-lint that append text after the SARIF JSON.
func isSARIF(data []byte) bool {
	// Check if data starts with '{' (JSON object)
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return false
	}

	// Find the end of the JSON object by counting braces
	depth := 0
	jsonEnd := -1
	for i, b := range data {
		if b == '{' {
			depth++
		} else if b == '}' {
			depth--
			if depth == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}

	if jsonEnd <= 0 {
		return false
	}

	// Parse just the JSON portion
	var probe struct {
		Version string `json:"version"`
		Schema  string `json:"$schema"`
	}
	if err := json.Unmarshal(data[:jsonEnd], &probe); err != nil {
		return false
	}
	return probe.Version != "" || probe.Schema != ""
}

// extractSARIF extracts just the SARIF JSON from data that may have trailing text.
func extractSARIF(data []byte) []byte {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return nil
	}

	// Find the end of the JSON object
	depth := 0
	for i, b := range data {
		if b == '{' {
			depth++
		} else if b == '}' {
			depth--
			if depth == 0 {
				return data[:i+1]
			}
		}
	}
	return nil
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

	for scanner.Scan() {
		line := scanner.Text()
		lineType, lineContext := p.patternMatcher.ClassifyOutputLine(line, command, args)
		task.AddOutputLine(line, lineType, lineContext)
	}
}

// ProcessStream processes input from an io.Reader line-by-line, calling onLine
// for each classified line. This enables live rendering as output arrives.
func (p *Processor) ProcessStream(
	input io.Reader,
	command string,
	args []string,
	onLine LineCallback,
) error {
	scanner := bufio.NewScanner(input)
	buf := make([]byte, 0, bufio.MaxScanTokenSize)
	scanner.Buffer(buf, p.maxLineLength)

	for scanner.Scan() {
		line := scanner.Text()
		lineType, lineContext := p.patternMatcher.ClassifyOutputLine(line, command, args)
		onLine(line, lineType, lineContext)
	}

	return scanner.Err()
}

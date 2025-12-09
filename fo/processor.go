// Package fo provides a processor for handling command output.
package fo

import (
	"bufio"
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

// ProcessOutput processes buffered output with line-by-line classification.
func (p *Processor) ProcessOutput(
	task *design.Task,
	output []byte,
	command string,
	args []string,
) {
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

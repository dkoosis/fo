// Package adapter provides interfaces for parsing tool-specific output into structured events.
// This allows fo to support arbitrary tools by implementing custom parsers.
package adapter

// Event represents a parsed event from tool output.
type Event struct {
	Type     string                 // "progress", "error", "metric", "warning", "success", "info"
	Message  string                 // Human-readable message
	Metadata map[string]interface{} // Structured data associated with the event
}

// Parser defines the interface for tool-specific output parsers.
// Implement this interface to add support for new tools with custom output formats.
//
// Example usage:
//
//	type GoTestParser struct {
//	    // parser state...
//	}
//
//	func (p *GoTestParser) ParseLine(line string) (Event, bool) {
//	    // Parse line and return Event if it matches a pattern
//	    if strings.HasPrefix(line, "FAIL") {
//	        return Event{Type: "error", Message: line}, true
//	    }
//	    return Event{}, false
//	}
//
//	func (p *GoTestParser) Finalize() interface{} {
//	    // Return summary statistics
//	    return map[string]interface{}{
//	        "total_tests": p.totalTests,
//	        "passed": p.passed,
//	        "failed": p.failed,
//	    }
//	}
type Parser interface {
	// ParseLine ingests a single line of output and returns:
	// - An Event if the line was recognized and parsed
	// - A boolean indicating whether the line was consumed/handled by this parser
	//
	// If the line is not relevant to this parser, return (Event{}, false).
	// If the line is parsed successfully, return (Event{...}, true).
	ParseLine(line string) (Event, bool)

	// Finalize is called after all output has been processed.
	// It returns structured data summarizing the parsed results.
	// The return value can be any type that makes sense for the parser's domain
	// (e.g., test statistics, build metrics, lint results).
	//
	// Return nil if there's no summary data to provide.
	Finalize() interface{}
}

// PassthroughParser is a default parser that passes all lines through without modification.
// Use this when you don't need custom parsing logic.
type PassthroughParser struct{}

// ParseLine implements Parser interface with passthrough behavior.
func (p *PassthroughParser) ParseLine(line string) (Event, bool) {
	return Event{
		Type:     "info",
		Message:  line,
		Metadata: nil,
	}, true
}

// Finalize implements Parser interface with no summary.
func (p *PassthroughParser) Finalize() interface{} {
	return nil
}

// Multi-tool delimiter protocol — multiplexes several tool outputs into a
// single stdin stream via lines of the form:
//
//	--- tool:<name> format:<sarif|testjson> ---
//
// Restored for fo-5b6 after the v2 cutover removed internal/report.
package report

import (
	"bytes"
	"errors"
	"regexp"
)

// ErrNoSections is returned when ParseSections finds no delimiter lines.
var ErrNoSections = errors.New("no sections found in report input")

var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson) ---$`,
)

// IsDelimiter reports whether line is a section delimiter.
func IsDelimiter(line []byte) bool {
	return delimiterRe.Match(line)
}

// HasDelimiter reports whether data begins with (after optional leading
// whitespace) a section delimiter line. Used to decide whether to dispatch
// to the multiplexer instead of the single-stream parser.
func HasDelimiter(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return false
	}
	first := trimmed
	if i := bytes.IndexAny(trimmed, "\r\n"); i >= 0 {
		first = trimmed[:i]
	}
	return IsDelimiter(first)
}

// Section is one tool's output within a multiplexed report.
type Section struct {
	Tool    string
	Format  string
	Content []byte
}

// ParseSections splits delimited input into sections. Lines before the first
// delimiter are silently discarded. CRLF is normalized to LF.
func ParseSections(data []byte) ([]Section, error) {
	nl := []byte{'\n'}
	data = bytes.ReplaceAll(data, []byte("\r\n"), nl)
	data = bytes.TrimSuffix(data, nl)
	lines := bytes.Split(data, nl)

	var sections []Section
	var current *Section

	for _, line := range lines {
		if m := delimiterRe.FindSubmatch(line); m != nil {
			if current != nil {
				current.Content = bytes.TrimSuffix(current.Content, nl)
				sections = append(sections, *current)
			}
			current = &Section{
				Tool:   string(m[1]),
				Format: string(m[2]),
			}
			continue
		}
		if current != nil {
			current.Content = append(current.Content, line...)
			current.Content = append(current.Content, '\n')
		}
	}
	if current != nil {
		current.Content = bytes.TrimSuffix(current.Content, nl)
		sections = append(sections, *current)
	}

	if len(sections) == 0 {
		return nil, ErrNoSections
	}
	return sections, nil
}

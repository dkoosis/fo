// Multi-tool delimiter protocol — multiplexes several tool outputs into a
// single stdin stream via lines of the form:
//
//	--- tool:<name> format:<sarif|testjson> [status:<value>] ---
//
// The optional status attribute signals the tool's execution outcome to the
// renderer. Valid values: ok, clean, partial, timeout, skipped, error.
// Omitting status is equivalent to ok.
//
// Restored for fo-5b6 after the v2 cutover removed internal/report.
package report

import (
	"bytes"
	"errors"
	"regexp"
)

// Section status values.
const (
	StatusOK      = "ok"
	StatusClean   = "clean"
	StatusPartial = "partial"
	StatusTimeout = "timeout"
	StatusSkipped = "skipped"
	StatusError   = "error"
)

// ErrNoSections is returned when ParseSections finds no delimiter lines.
var ErrNoSections = errors.New("no sections found in report input")

var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson)(?: status:(\w+))? ---$`,
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
// Status carries the optional status attribute from the delimiter line;
// empty string means the attribute was absent (treated as ok).
type Section struct {
	Tool    string
	Format  string
	Status  string
	Content []byte
}

// ParseSections splits delimited input into sections. CRLF is normalized to
// LF. Any non-whitespace lines preceding the first delimiter are returned
// in prelude so callers can surface them; whitespace-only preludes yield a
// nil prelude. Dropping the prelude silently would mask wrapper bugs that
// emit banners ahead of the first tool's output (fo-qhi).
func ParseSections(data []byte) (sections []Section, prelude []byte, err error) {
	nl := []byte{'\n'}
	data = bytes.ReplaceAll(data, []byte("\r\n"), nl)
	data = bytes.TrimSuffix(data, nl)
	lines := bytes.Split(data, nl)

	var current *Section
	var preludeBuf []byte

	for _, line := range lines {
		if m := delimiterRe.FindSubmatch(line); m != nil {
			if current != nil {
				current.Content = bytes.TrimSuffix(current.Content, nl)
				sections = append(sections, *current)
			}
			current = &Section{
				Tool:   string(m[1]),
				Format: string(m[2]),
				Status: string(m[3]),
			}
			continue
		}
		if current != nil {
			current.Content = append(current.Content, line...)
			current.Content = append(current.Content, '\n')
			continue
		}
		preludeBuf = append(preludeBuf, line...)
		preludeBuf = append(preludeBuf, '\n')
	}
	if current != nil {
		current.Content = bytes.TrimSuffix(current.Content, nl)
		sections = append(sections, *current)
	}

	if len(bytes.TrimSpace(preludeBuf)) > 0 {
		prelude = bytes.TrimSuffix(preludeBuf, nl)
	}

	if len(sections) == 0 {
		return nil, prelude, ErrNoSections
	}
	return sections, prelude, nil
}

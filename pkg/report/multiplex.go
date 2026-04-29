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
	"fmt"
	"regexp"
	"strings"
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

// SupportedFormats is the list of format values fo accepts in delimiter lines.
var SupportedFormats = []string{"sarif", "testjson"}

var (
	delimiterRe = regexp.MustCompile(
		`^--- tool:(\w[\w-]*) format:(sarif|testjson)(?: status:(\w+))? ---$`,
	)
	// delimiterShapeRe matches the delimiter shape with any word for format,
	// so we can distinguish "no delimiter" from "delimiter with unknown format".
	delimiterShapeRe = regexp.MustCompile(
		`^--- tool:(\w[\w-]*) format:([\w-]+)(?: status:(\w+))? ---$`,
	)
)

// UnknownFormatError is returned by ParseSections when a delimiter has the
// expected shape but its format value is not in SupportedFormats.
type UnknownFormatError struct {
	SectionIndex int // 1-based position of the offending section
	Line         string
	Tool         string
	Format       string
}

func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf(
		"section %d: unknown format %q for tool %q in delimiter %q (supported: %s)",
		e.SectionIndex, e.Format, e.Tool, e.Line, strings.Join(SupportedFormats, ", "),
	)
}

// IsDelimiter reports whether line is a valid section delimiter (recognized
// format value).
func IsDelimiter(line []byte) bool {
	return delimiterRe.Match(line)
}

// IsDelimiterShape reports whether line has the shape of a section delimiter,
// regardless of whether the format value is recognized.
func IsDelimiterShape(line []byte) bool {
	return delimiterShapeRe.Match(line)
}

// HasDelimiter reports whether data begins with (after optional leading
// whitespace) a section delimiter line. Used to decide whether to dispatch
// to the multiplexer instead of the single-stream parser. Accepts any
// shape-matching delimiter so an unknown format value routes to the
// multiplexer (which surfaces a precise error) rather than falling through
// to the generic 'unrecognized input' path.
func HasDelimiter(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return false
	}
	first := trimmed
	if i := bytes.IndexAny(trimmed, "\r\n"); i >= 0 {
		first = trimmed[:i]
	}
	return IsDelimiterShape(first)
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
	sectionIndex := 0

	for _, line := range lines {
		if m := delimiterRe.FindSubmatch(line); m != nil {
			if current != nil {
				current.Content = bytes.TrimSuffix(current.Content, nl)
				sections = append(sections, *current)
			}
			sectionIndex++
			current = &Section{
				Tool:   string(m[1]),
				Format: string(m[2]),
				Status: string(m[3]),
			}
			continue
		}
		if m := delimiterShapeRe.FindSubmatch(line); m != nil {
			return nil, nil, &UnknownFormatError{
				SectionIndex: sectionIndex + 1,
				Line:         string(line),
				Tool:         string(m[1]),
				Format:       string(m[2]),
			}
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

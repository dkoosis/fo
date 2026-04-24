// Package report parses fo's multi-tool delimiter protocol.
//
// The protocol multiplexes multiple tools' output into a single stdin stream
// via delimiter lines of the form:
//
//	--- tool:<name> format:<sarif|testjson> [status:<status>] ---
//
// The optional status attribute (fo-s76) expresses per-tool outcome so agents
// cannot mistake a silent crash for a clean pass. Allowed values:
//
//	ok       — tool ran to completion, findings parsed
//	clean    — tool ran, zero findings
//	partial  — tool ran but output was incomplete (e.g. truncated)
//	timeout  — tool exceeded its time budget
//	skipped  — tool was not on PATH or was otherwise skipped
//	error    — tool crashed or its output could not be parsed
//
// Backward compatibility: delimiters without a status attribute continue to
// parse. Downstream mapping code assigns a default (typically "ok" when
// findings parsed successfully, "error" on parse failure).
package report

import (
	"bytes"
	"errors"
	"regexp"
)

// ErrNoSections is returned when Parse finds no delimiter lines in the input.
var ErrNoSections = errors.New("no sections found in report input")

// Status is the per-tool outcome carried on a section delimiter.
type Status string

// Allowed status values. Empty string is valid and means "unspecified" — the
// caller/mapper derives a default.
const (
	StatusOK      Status = "ok"
	StatusClean   Status = "clean"
	StatusPartial Status = "partial"
	StatusTimeout Status = "timeout"
	StatusSkipped Status = "skipped"
	StatusError   Status = "error"
)

// validStatuses enumerates the set of accepted status tokens on delimiter lines.
var validStatuses = map[string]Status{
	"ok":      StatusOK,
	"clean":   StatusClean,
	"partial": StatusPartial,
	"timeout": StatusTimeout,
	"skipped": StatusSkipped,
	"error":   StatusError,
}

// IsValidStatus reports whether s is a recognized status token.
func IsValidStatus(s string) bool {
	_, ok := validStatuses[s]
	return ok
}

// delimiterRe matches report section delimiter lines, with an optional
// status:X attribute after the format attribute.
var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson)(?: status:(ok|clean|partial|timeout|skipped|error))? ---$`,
)

// IsDelimiter reports whether line is a section delimiter.
func IsDelimiter(line []byte) bool {
	return delimiterRe.Match(line)
}

// Section represents one tool's output within a report.
type Section struct {
	Tool    string // e.g. "lint", "test", "vuln"
	Format  string // "sarif" or "testjson"
	Status  Status // optional per-tool status; empty if not present on delimiter
	Content []byte // raw tool output
}

// Parse splits delimited report input into sections.
// Lines before the first delimiter are silently discarded.
func Parse(data []byte) ([]Section, error) {
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
				Status: Status(m[3]), // empty when absent
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

package report

import (
	"bytes"
	"errors"
	"regexp"
)

// ErrNoSections is returned when Parse finds no delimiter lines in the input.
var ErrNoSections = errors.New("no sections found in report input")

// delimiterRe matches report section delimiter lines.
var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson|text|metrics|archlint|jscpd)(?: status:(pass|fail))? ---$`,
)

// IsDelimiter reports whether line is a section delimiter.
func IsDelimiter(line []byte) bool {
	return delimiterRe.Match(line)
}

var newline = []byte("\n")

// Section represents one tool's output within a report.
type Section struct {
	Tool    string // e.g. "lint", "test", "vuln"
	Format  string // "sarif", "testjson", "text", "metrics", "archlint", "jscpd"
	Status  string // "pass" or "fail" (required for text, derived for others)
	Content []byte // raw tool output
}

// Parse splits delimited report input into sections.
// Lines before the first delimiter are silently discarded.
func Parse(data []byte) ([]Section, error) {
	data = bytes.TrimSuffix(data, newline)
	lines := bytes.Split(data, newline)
	var sections []Section
	var current *Section

	for _, line := range lines {
		if m := delimiterRe.FindSubmatch(line); m != nil {
			if current != nil {
				current.Content = bytes.TrimSuffix(current.Content, newline)
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
		}
	}
	if current != nil {
		current.Content = bytes.TrimSuffix(current.Content, newline)
		sections = append(sections, *current)
	}

	if len(sections) == 0 {
		return nil, ErrNoSections
	}
	return sections, nil
}
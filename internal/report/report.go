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
	`^--- tool:(\w[\w-]*) format:(sarif|testjson) ---$`,
)

// IsDelimiter reports whether line is a section delimiter.
func IsDelimiter(line []byte) bool {
	return delimiterRe.Match(line)
}

// Section represents one tool's output within a report.
type Section struct {
	Tool    string // e.g. "lint", "test", "vuln"
	Format  string // "sarif" or "testjson"
	Content []byte // raw tool output
}

// Parse splits delimited report input into sections.
// Lines before the first delimiter are silently discarded.
func Parse(data []byte) ([]Section, error) {
	nl := []byte{'\n'}
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
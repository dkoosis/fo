package report

import (
	"bytes"
	"fmt"
	"regexp"
)

// DelimiterRe matches report section delimiter lines.
// Canonical regex — used by both the section parser and format detection.
var DelimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson|text|metrics|archlint|jscpd)(?: status:(pass|fail))? ---$`,
)

// Section represents one tool's output within a report.
type Section struct {
	Tool    string // e.g. "lint", "test", "vuln"
	Format  string // "sarif", "testjson", "text", "metrics", "archlint", "jscpd"
	Status  string // "pass" or "fail" (required for text, derived for others)
	Content []byte // raw tool output
}

// Parse splits delimited report input into sections.
func Parse(data []byte) ([]Section, error) {
	data = trimTrailingNewline(data)
	lines := bytes.Split(data, []byte("\n"))
	var sections []Section
	var current *Section

	for _, line := range lines {
		if m := DelimiterRe.FindSubmatch(line); m != nil {
			if current != nil {
				current.Content = trimTrailingNewline(current.Content)
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
		current.Content = trimTrailingNewline(current.Content)
		sections = append(sections, *current)
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections found in report input")
	}
	return sections, nil
}

// trimTrailingNewline removes exactly one trailing newline byte, if present.
func trimTrailingNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}

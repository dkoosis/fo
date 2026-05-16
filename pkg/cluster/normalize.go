package cluster

import (
	"regexp"
	"strings"
)

// Rule order is load-bearing: each later pattern would consume
// substrings the earlier one wants to claim. See plan §1.5.
//
// Rule 6 (tmp) must run before rule 4 (path) because /var/folders/... is
// also a valid POSIX path; rule 4 must run before rule 9 (line:col)
// and rule 10 (bare number) so that the digits embedded in a path do
// not get replaced first.

var normRules = []struct {
	name string
	re   *regexp.Regexp
	rep  string
}{
	{"uuid", regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), "<UUID>"},
	{"hash", regexp.MustCompile(`\b[0-9a-fA-F]{32,64}\b`), "<HASH>"},
	{"addr", regexp.MustCompile(`0x[0-9a-fA-F]+`), "<ADDR>"},
	{"tmp", regexp.MustCompile(`(?:/tmp|/var/folders/[^/\s]+/[^/\s]+)/[A-Za-z0-9._/-]*`), "<TMP>"},
	{"winpath", regexp.MustCompile(`[A-Za-z]:\\(?:[^\s\\]+\\)+[^\s\\]+\.(?:go|txt|json|yaml|yml|toml)`), "<PATH>"},
	{"path", regexp.MustCompile(`/(?:[^\s:/]+/)+[^\s:/]+\.(?:go|txt|json|yaml|yml|toml)`), "<PATH>"},
	{"dur", regexp.MustCompile(`\b\d+(?:\.\d+)?(?:ns|µs|us|ms|s|m|h)\b`), "<DUR>"},
	{"ts", regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?`), "<TS>"},
	{"linecol", regexp.MustCompile(`\b\d+:\d+\b`), "<L:C>"},
	{"num", regexp.MustCompile(`-?\b\d+(?:\.\d+)?\b`), "<N>"},
}

var wsRun = regexp.MustCompile(`[ \t]+`)

var (
	panicHeader        = regexp.MustCompile(`(?m)^panic:\s*(.*)$`)
	goroutineHeader    = regexp.MustCompile(`(?m)^goroutine\s+\d+\s+\[`)
	testifyErrorPrefix = regexp.MustCompile(`(?m)^\s*Error:\s*(.*)$`)
	testifyTestLine    = regexp.MustCompile(`(?m)^\s*Test:\s`)
)

// extractAnchor picks the most identifying single line of failure
// output. Order: panic message, testify Error:, first line with a
// colon, first non-empty line. Returns "" if input is empty/blank.
func extractAnchor(output string, maxLen int) string {
	if maxLen > 0 && len(output) > maxLen*8 {
		// Pathological input: hard cap before scanning. Multiplier
		// keeps room for many lines so the picker still has choices.
		output = output[:maxLen*8]
	}

	if m := panicHeader.FindStringSubmatchIndex(output); m != nil {
		msgStart := m[2]
		// Take everything between panic: and the next goroutine
		// header (or end-of-string).
		tail := output[msgStart:]
		if g := goroutineHeader.FindStringIndex(tail); g != nil {
			tail = tail[:g[0]]
		}
		anchor := firstNonEmptyLine(tail)
		if anchor != "" {
			return truncate(anchor, maxLen)
		}
	}

	if m := testifyErrorPrefix.FindStringSubmatchIndex(output); m != nil {
		// Take lines after Error: up to blank or Test: marker.
		tail := output[m[2]:]
		end := len(tail)
		if t := testifyTestLine.FindStringIndex(tail); t != nil && t[0] < end {
			end = t[0]
		}
		if blank := strings.Index(tail[:end], "\n\n"); blank >= 0 {
			end = blank
		}
		anchor := strings.TrimSpace(tail[:end])
		anchor = firstNonEmptyLine(anchor)
		if anchor != "" {
			return truncate(anchor, maxLen)
		}
	}

	// First non-empty line containing a colon.
	for _, line := range strings.Split(output, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if strings.Contains(l, ":") {
			return truncate(l, maxLen)
		}
	}

	// Fall back to first non-empty line.
	return truncate(firstNonEmptyLine(output), maxLen)
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if l != "" {
			return l
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// Normalize collapses dynamic content in assertion text so two
// failures that differ only in line numbers, addresses, durations,
// etc. share a signature. Idempotent: Normalize(Normalize(s)) ==
// Normalize(s).
func Normalize(s string) string {
	for _, r := range normRules {
		s = r.re.ReplaceAllString(s, r.rep)
	}
	s = wsRun.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

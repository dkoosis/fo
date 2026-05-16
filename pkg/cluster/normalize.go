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

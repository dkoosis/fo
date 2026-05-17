// Package suppress parses fo's .fo/ignore suppression file — a
// line-based list of rule suppressions used to silence individual
// findings during diff classification and rendering.
//
// Format (one suppression per line):
//
//	<rule_id> [glob=<pattern>] [until=<YYYY-MM-DD>] [reason=<quoted-or-bareword>]
//
// Examples:
//
//	SA1019 until=2026-12-31 reason="upstream not migrated yet"
//	G115 glob=internal/legacy/**
//	govet:shadow glob=cmd/** until=2026-06-01
//
// Defaults: missing glob → "**" (matches everything). Missing until →
// never expires. Missing reason → empty. Lines beginning with `#` and
// blank lines are ignored.
//
// Format choice (line-based, not TOML): .fo/ignore is hand-edited by
// humans and read by agents alongside fo's stdout. Line-based keeps it
// grep-friendly, free of nesting, and matches the conventions of
// pkg/status and pkg/tally.
package suppress

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// DefaultGlob is applied when a suppression line omits a `glob=` key.
const DefaultGlob = "**"

// Suppression is one parsed entry from .fo/ignore.
type Suppression struct {
	RuleID string     `json:"rule_id"`
	Glob   string     `json:"glob"`
	Until  *time.Time `json:"until,omitempty"`
	Reason string     `json:"reason,omitempty"`
	Line   int        `json:"line"`
}

// Expired reports whether the suppression's until date is in the past
// relative to now. Suppressions without an until are never expired.
func (s Suppression) Expired(now time.Time) bool {
	if s.Until == nil {
		return false
	}
	// Until is inclusive: suppression is valid through the end of the
	// Until day (UTC). Compare day-to-day, not instant-to-instant.
	y, m, d := now.UTC().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return today.After(*s.Until)
}

// Format renders s as a canonical .fo/ignore line. Keys are emitted in
// glob, until, reason order; defaults are omitted. Suitable for
// round-trip: Parse(Format(s)) yields an equivalent Suppression.
func (s Suppression) Format() string {
	var b strings.Builder
	b.WriteString(s.RuleID)
	if s.Glob != "" && s.Glob != DefaultGlob {
		b.WriteString(" glob=")
		writeValue(&b, s.Glob)
	}
	if s.Until != nil {
		b.WriteString(" until=")
		b.WriteString(s.Until.Format("2006-01-02"))
	}
	if s.Reason != "" {
		b.WriteString(" reason=")
		writeValue(&b, s.Reason)
	}
	return b.String()
}

var valueEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// writeValue emits v, quoting and escaping if it contains whitespace,
// quotes, or backslashes that would otherwise round-trip incorrectly.
func writeValue(b *strings.Builder, v string) {
	if strings.ContainsAny(v, " \t\"\\") {
		b.WriteString(`"`)
		b.WriteString(valueEscaper.Replace(v))
		b.WriteString(`"`)
		return
	}
	b.WriteString(v)
}

// Sentinel errors. Parse failures wrap one of these so callers can
// errors.Is for category checks.
var (
	ErrMalformedLine = errors.New("suppress: malformed line")
	ErrMissingRuleID = errors.New("suppress: missing rule_id")
	ErrInvalidDate   = errors.New("suppress: invalid until date")
	ErrUnknownKey    = errors.New("suppress: unknown key")
	ErrUnclosedQuote = errors.New("suppress: unclosed quoted value")
)

// Parse reads .fo/ignore content from r and returns the parsed
// suppressions. Returns the first parse error encountered, pinned to a
// line number. Blank lines and `#`-prefixed comment lines are skipped.
func Parse(r io.Reader) ([]Suppression, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)

	var out []Suppression
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("suppress: line %d: %w", lineNo, err)
		}
		s.Line = lineNo
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("suppress: read: %w", err)
	}
	return out, nil
}

func parseLine(line string) (Suppression, error) {
	toks, err := tokenize(line)
	if err != nil {
		return Suppression{}, err
	}
	if len(toks) == 0 {
		return Suppression{}, ErrMissingRuleID
	}
	if strings.ContainsRune(toks[0], '=') {
		return Suppression{}, fmt.Errorf("%w: first token %q looks like a key=value", ErrMissingRuleID, toks[0])
	}

	s := Suppression{RuleID: toks[0], Glob: DefaultGlob}
	if s.RuleID == "" {
		return Suppression{}, ErrMissingRuleID
	}

	for _, tok := range toks[1:] {
		eq := strings.IndexByte(tok, '=')
		if eq <= 0 {
			return Suppression{}, fmt.Errorf("%w: expected key=value, got %q", ErrMalformedLine, tok)
		}
		key := strings.ToLower(tok[:eq])
		val := tok[eq+1:]
		switch key {
		case "glob":
			if val == "" {
				return Suppression{}, fmt.Errorf("%w: empty glob value", ErrMalformedLine)
			}
			s.Glob = val
		case "until":
			t, perr := time.Parse("2006-01-02", val)
			if perr != nil {
				return Suppression{}, fmt.Errorf("%w: %q", ErrInvalidDate, val)
			}
			// Reject zero-year. time.Parse accepts "0001-01-01"; if a
			// caller hands us that (deserialization bug, default-zero
			// time), Expired returns true for every call, silently
			// disabling the rule (fo-7jv).
			if t.Year() <= 1 {
				return Suppression{}, fmt.Errorf("%w: zero-year %q", ErrInvalidDate, val)
			}
			s.Until = &t
		case "reason":
			s.Reason = val
		default:
			return Suppression{}, fmt.Errorf("%w: %q", ErrUnknownKey, key)
		}
	}
	return s, nil
}

// tokenize splits a line into whitespace-separated tokens, honoring
// double-quoted values (which may contain spaces and \" escapes). The
// quoted region must follow an `=` — quotes inside barewords are not
// treated specially. Returned tokens are key=value or bare rule_id; the
// surrounding quotes are stripped.
func tokenize(line string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	st := tokState{}
	for i := range len(line) {
		if err := st.step(line[i], &cur, &toks); err != nil {
			return nil, err
		}
	}
	if st.inQuote {
		return nil, ErrUnclosedQuote
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks, nil
}

type tokState struct {
	inQuote bool
	escape  bool
}

func (st *tokState) step(c byte, cur *strings.Builder, toks *[]string) error {
	if st.escape {
		cur.WriteByte(c)
		st.escape = false
		return nil
	}
	if st.inQuote {
		switch c {
		case '\\':
			st.escape = true
		case '"':
			st.inQuote = false
		default:
			cur.WriteByte(c)
		}
		return nil
	}
	switch c {
	case ' ', '\t':
		if cur.Len() > 0 {
			*toks = append(*toks, cur.String())
			cur.Reset()
		}
	case '"':
		s := cur.String()
		if len(s) == 0 || s[len(s)-1] != '=' {
			return fmt.Errorf("%w: stray '\"' outside key=value", ErrMalformedLine)
		}
		st.inQuote = true
	default:
		cur.WriteByte(c)
	}
	return nil
}

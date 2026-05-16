package suppress

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const ruleSA1019 = "SA1019"

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

func TestParse_fullLine(t *testing.T) {
	in := `SA1019 glob=internal/legacy/** until=2026-12-31 reason="upstream not migrated yet"`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d suppressions, want 1", len(got))
	}
	s := got[0]
	if s.RuleID != ruleSA1019 {
		t.Errorf("RuleID = %q", s.RuleID)
	}
	if s.Glob != "internal/legacy/**" {
		t.Errorf("Glob = %q", s.Glob)
	}
	if s.Until == nil || !s.Until.Equal(mustDate(t, "2026-12-31")) {
		t.Errorf("Until = %v", s.Until)
	}
	if s.Reason != "upstream not migrated yet" {
		t.Errorf("Reason = %q", s.Reason)
	}
	if s.Line != 1 {
		t.Errorf("Line = %d", s.Line)
	}
}

func TestParse_ruleIDOnly_defaults(t *testing.T) {
	got, err := Parse(strings.NewReader("G115\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
	s := got[0]
	if s.RuleID != "G115" {
		t.Errorf("RuleID = %q", s.RuleID)
	}
	if s.Glob != DefaultGlob {
		t.Errorf("Glob = %q, want %q", s.Glob, DefaultGlob)
	}
	if s.Until != nil {
		t.Errorf("Until = %v, want nil", s.Until)
	}
	if s.Reason != "" {
		t.Errorf("Reason = %q", s.Reason)
	}
}

func TestParse_commentsAndBlanks(t *testing.T) {
	in := `# top comment

# another
SA1019
   # indented comment

G115 glob=cmd/**
`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	if got[0].RuleID != ruleSA1019 || got[1].RuleID != "G115" || got[1].Glob != "cmd/**" {
		t.Errorf("rows = %+v", got)
	}
}

func TestParse_multipleEntries_lineNumbers(t *testing.T) {
	in := "SA1019\n\n# c\nG115 until=2026-06-01\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].Line != 1 {
		t.Errorf("got[0].Line = %d, want 1", got[0].Line)
	}
	if got[1].Line != 4 {
		t.Errorf("got[1].Line = %d, want 4", got[1].Line)
	}
}

func TestParse_whitespaceAndCase(t *testing.T) {
	in := "   SA1019   GLOB=foo/**   Until=2026-06-01   \n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].Glob != "foo/**" {
		t.Errorf("Glob = %q", got[0].Glob)
	}
	if got[0].Until == nil || !got[0].Until.Equal(mustDate(t, "2026-06-01")) {
		t.Errorf("Until = %v", got[0].Until)
	}
}

func TestParse_errors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"missing rule_id (key-only line)", "glob=foo/**\n", ErrMissingRuleID},
		{"malformed until date", "SA1019 until=2026-13-40\n", ErrInvalidDate},
		{"until not a date", "SA1019 until=soon\n", ErrInvalidDate},
		{"unknown key", "SA1019 severity=high\n", ErrUnknownKey},
		{"unclosed quote", `SA1019 reason="never closed` + "\n", ErrUnclosedQuote},
		{"stray equals as token", "SA1019 =value\n", ErrMalformedLine},
		{"bare token after rule", "SA1019 noequals\n", ErrMalformedLine},
		{"empty glob", "SA1019 glob=\n", ErrMalformedLine},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(c.in))
			if err == nil {
				t.Fatalf("got nil err, want %v", c.want)
			}
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want Is %v", err, c.want)
			}
		})
	}
}

func TestParse_quotedReasonWithEscapes(t *testing.T) {
	in := `SA1019 reason="he said \"hi\" then left"` + "\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := `he said "hi" then left`
	if got[0].Reason != want {
		t.Errorf("Reason = %q, want %q", got[0].Reason, want)
	}
}

func TestParse_barewordReason(t *testing.T) {
	got, err := Parse(strings.NewReader("SA1019 reason=legacy\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].Reason != "legacy" {
		t.Errorf("Reason = %q", got[0].Reason)
	}
}

func TestParse_empty(t *testing.T) {
	got, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d, want 0", len(got))
	}
}

func TestExpired(t *testing.T) {
	past := mustDate(t, "2020-01-01")
	future := mustDate(t, "2099-01-01")
	now := mustDate(t, "2026-05-16")

	cases := []struct {
		name string
		s    Suppression
		want bool
	}{
		{"no until → never expired", Suppression{}, false},
		{"past until → expired", Suppression{Until: &past}, true},
		{"future until → not expired", Suppression{Until: &future}, false},
		{"same day → not expired", Suppression{Until: &now}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.s.Expired(now); got != c.want {
				t.Errorf("Expired = %v, want %v", got, c.want)
			}
		})
	}
}

func TestFormat_roundTrip(t *testing.T) {
	until := mustDate(t, "2026-12-31")
	cases := []Suppression{
		{RuleID: ruleSA1019},
		{RuleID: "G115", Glob: "internal/legacy/**"},
		{RuleID: ruleSA1019, Glob: DefaultGlob, Until: &until, Reason: "upstream not migrated yet"},
		{RuleID: "govet:shadow", Glob: "cmd/**", Until: &until},
		{RuleID: "X", Reason: "plain"},
		{RuleID: "X", Reason: `has "quotes" and spaces`},
	}
	for _, want := range cases {
		t.Run(want.RuleID+"/"+want.Reason, func(t *testing.T) {
			if want.Glob == "" {
				want.Glob = DefaultGlob
			}
			line := want.Format()
			got, err := Parse(strings.NewReader(line + "\n"))
			if err != nil {
				t.Fatalf("Parse(%q): %v", line, err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d", len(got))
			}
			g := got[0]
			if g.RuleID != want.RuleID {
				t.Errorf("RuleID: got %q want %q (line=%q)", g.RuleID, want.RuleID, line)
			}
			if g.Glob != want.Glob {
				t.Errorf("Glob: got %q want %q (line=%q)", g.Glob, want.Glob, line)
			}
			if (g.Until == nil) != (want.Until == nil) {
				t.Errorf("Until nilness mismatch (line=%q)", line)
			} else if g.Until != nil && !g.Until.Equal(*want.Until) {
				t.Errorf("Until: got %v want %v", g.Until, want.Until)
			}
			if g.Reason != want.Reason {
				t.Errorf("Reason: got %q want %q (line=%q)", g.Reason, want.Reason, line)
			}
		})
	}
}

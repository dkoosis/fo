package suppress

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**", "anything/at/all.go", true},
		{"**", "", true},
		{globLegacyStar, "internal/legacy/foo.go", true},
		{globLegacyStar, "internal/legacy/sub/bar.go", true},
		{globLegacyStar, "internal/other/foo.go", false},
		{globCmdStar, "cmd/fo/main.go", true},
		{globCmdStar, "pkg/cmd/foo.go", false},
		{"*.go", "main.go", true},
		{"*.go", "pkg/main.go", false},
		{"pkg/*/foo.go", "pkg/bar/foo.go", true},
		{"pkg/*/foo.go", "pkg/bar/baz/foo.go", false},
		{"exact.go", "exact.go", true},
		{"exact.go", "exact.gox", false},
		{"a?c", "abc", true},
		{"a?c", "a/c", false},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestRulesetMatch(t *testing.T) {
	rs := NewRuleset([]Suppression{
		{RuleID: "SA1019", Glob: "**"},
		{RuleID: ruleG115, Glob: globLegacyStar},
	})

	if i := rs.Match("SA1019", "anywhere/file.go"); i != 0 {
		t.Errorf("SA1019 anywhere: got %d, want 0", i)
	}
	if i := rs.Match(ruleG115, "internal/legacy/x.go"); i != 1 {
		t.Errorf("G115 legacy: got %d, want 1", i)
	}
	if i := rs.Match(ruleG115, "internal/modern/x.go"); i != -1 {
		t.Errorf("G115 modern: got %d, want -1", i)
	}
	if i := rs.Match("UNKNOWN", "x.go"); i != -1 {
		t.Errorf("UNKNOWN: got %d, want -1", i)
	}

	var nilRS *Ruleset
	if i := nilRS.Match("X", "y"); i != -1 {
		t.Errorf("nil ruleset: got %d, want -1", i)
	}
}

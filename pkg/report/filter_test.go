package report

import (
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/suppress"
)

const ruleSA1019 = "SA1019"

func mustDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &t
}

// TestApplyFilter_ClearsSuppressedTail verifies that dropped Findings
// don't stay pinned in the slice's backing array after filtering.
// Regression for fo-zp0: ApplyFilter previously left suppressed structs
// in r.Findings[len(kept):cap], retaining their strings.
func TestApplyFilter_ClearsSuppressedTail(t *testing.T) {
	r := &Report{
		Findings: []Finding{
			{RuleID: ruleSA1019, File: "pkg/a.go", Message: "drop me"},
			{RuleID: "KEEP", File: "pkg/b.go", Message: "keep"},
		},
	}
	rs := suppress.NewRuleset([]suppress.Suppression{
		{RuleID: ruleSA1019, Glob: "**", Line: 1},
	})
	ApplyFilter(r, rs, time.Now())
	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings) = %d, want 1", len(r.Findings))
	}
	// Inspect the tail via slice reslice up to original cap. The
	// suppressed entry must be zero-valued — not still holding its
	// Message string.
	tail := r.Findings[:cap(r.Findings)][1:]
	if len(tail) == 0 {
		t.Skip("no tail capacity to verify")
	}
	if tail[0].RuleID != "" || tail[0].Message != "" {
		t.Errorf("suppressed tail not cleared: %+v", tail[0])
	}
}

func TestApplyFilter_ActiveRuleSuppresses(t *testing.T) {
	r := &Report{
		Findings: []Finding{
			{RuleID: ruleSA1019, File: "pkg/a.go", Severity: SeverityWarning, Message: "deprecated"},
			{RuleID: "G115", File: "internal/legacy/x.go", Severity: SeverityWarning, Message: "overflow"},
			{RuleID: "KEEP", File: "pkg/b.go", Severity: SeverityError, Message: "keep me"},
		},
	}
	rs := suppress.NewRuleset([]suppress.Suppression{
		{RuleID: ruleSA1019, Glob: "**", Line: 1},
		{RuleID: "G115", Glob: "internal/legacy/**", Line: 2},
	})
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	stats := ApplyFilter(r, rs, now)

	if len(r.Findings) != 1 || r.Findings[0].RuleID != "KEEP" {
		t.Errorf("findings after filter: %+v", r.Findings)
	}
	if r.Suppressed != 2 {
		t.Errorf("Suppressed = %d, want 2", r.Suppressed)
	}
	if stats.Total != 2 {
		t.Errorf("stats.Total = %d, want 2", stats.Total)
	}
	if stats.PerRule[0] != 1 || stats.PerRule[1] != 1 {
		t.Errorf("per-rule: %+v", stats.PerRule)
	}
	if len(r.Notices) != 0 {
		t.Errorf("unexpected notices: %v", r.Notices)
	}
}

// fo-f3u: a zero `now` would silently invert every expiry check
// (year-0001 is "before" every until-date, so expired rules look
// active). ApplyFilter defaults to time.Now() instead.
func TestApplyFilter_ZeroNowDoesNotInvertExpiry(t *testing.T) {
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	r := &Report{
		Findings: []Finding{
			{RuleID: ruleSA1019, File: "pkg/a.go", Severity: SeverityWarning, Message: "msg"},
		},
	}
	rs := suppress.NewRuleset([]suppress.Suppression{
		{RuleID: ruleSA1019, Glob: "**", Until: &past, Line: 1},
	})
	// Zero time. With no guard, Expired returns false → finding gets
	// suppressed; with the guard, the expired rule is honored as expired
	// → finding kept + notice emitted.
	stats := ApplyFilter(r, rs, time.Time{})
	if len(r.Findings) != 1 {
		t.Fatalf("findings after filter: got %d, want 1 (kept)", len(r.Findings))
	}
	if stats.Total != 0 {
		t.Errorf("stats.Total = %d, want 0", stats.Total)
	}
	if len(r.Notices) == 0 {
		t.Errorf("expected expiry notice, got none")
	}
}

func TestApplyFilter_ExpiredRuleKeepsAndNotices(t *testing.T) {
	r := &Report{
		Findings: []Finding{
			{RuleID: ruleSA1019, File: "a.go", Severity: SeverityWarning},
			{RuleID: ruleSA1019, File: "b.go", Severity: SeverityWarning},
		},
	}
	rs := suppress.NewRuleset([]suppress.Suppression{
		{RuleID: ruleSA1019, Glob: "**", Until: mustDate("2025-01-01"), Line: 7},
	})
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	stats := ApplyFilter(r, rs, now)

	if len(r.Findings) != 2 {
		t.Errorf("expired rule should keep findings: %+v", r.Findings)
	}
	if r.Suppressed != 0 {
		t.Errorf("Suppressed = %d, want 0", r.Suppressed)
	}
	if stats.Total != 0 {
		t.Errorf("stats.Total = %d, want 0", stats.Total)
	}
	if len(r.Notices) != 1 {
		t.Fatalf("want 1 notice, got %d: %v", len(r.Notices), r.Notices)
	}
	if !strings.Contains(r.Notices[0], "SA1019") || !strings.Contains(r.Notices[0], "expired") {
		t.Errorf("notice missing SA1019/expired: %q", r.Notices[0])
	}
	if !strings.Contains(r.Notices[0], "2025-01-01") {
		t.Errorf("notice missing date: %q", r.Notices[0])
	}
}

func TestApplyFilter_ActiveRuleBeatsEarlierExpired(t *testing.T) {
	r := &Report{
		Findings: []Finding{
			{RuleID: ruleSA1019, File: "pkg/a.go", Severity: SeverityWarning},
		},
	}
	rs := suppress.NewRuleset([]suppress.Suppression{
		{RuleID: ruleSA1019, Glob: "**", Until: mustDate("2025-01-01"), Line: 3},
		{RuleID: ruleSA1019, Glob: "**", Line: 7},
	})
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	stats := ApplyFilter(r, rs, now)

	if len(r.Findings) != 0 {
		t.Errorf("active rule should suppress despite earlier expired: %+v", r.Findings)
	}
	if stats.Total != 1 || stats.PerRule[1] != 1 {
		t.Errorf("expected active rule index 1 credited: stats=%+v", stats)
	}
	if len(r.Notices) != 0 {
		t.Errorf("no notice when active rule wins: %v", r.Notices)
	}
}

func TestApplyFilter_NoRulesetIsNoop(t *testing.T) {
	r := &Report{
		Findings: []Finding{{RuleID: "X", File: "a.go", Severity: SeverityWarning}},
	}
	stats := ApplyFilter(r, nil, time.Now())
	if len(r.Findings) != 1 {
		t.Errorf("nil ruleset must not drop findings")
	}
	if r.Suppressed != 0 || stats.Total != 0 {
		t.Errorf("nil ruleset must not increment counts")
	}

	empty := suppress.NewRuleset(nil)
	stats = ApplyFilter(r, empty, time.Now())
	if len(r.Findings) != 1 || r.Suppressed != 0 || stats.Total != 0 {
		t.Errorf("empty ruleset must be no-op")
	}
}

func TestApplyFilter_HitCounterAccumulates(t *testing.T) {
	r := &Report{
		Findings: []Finding{
			{RuleID: "R", File: "x.go"},
			{RuleID: "R", File: "y.go"},
			{RuleID: "R", File: "z.go"},
		},
		Suppressed: 5, // simulate prior accumulation
	}
	rs := suppress.NewRuleset([]suppress.Suppression{{RuleID: "R", Glob: "**"}})
	stats := ApplyFilter(r, rs, time.Now())
	if stats.Total != 3 {
		t.Errorf("stats.Total = %d, want 3", stats.Total)
	}
	if r.Suppressed != 8 {
		t.Errorf("Suppressed = %d, want 8 (5+3)", r.Suppressed)
	}
	if stats.PerRule[0] != 3 {
		t.Errorf("per-rule[0] = %d, want 3", stats.PerRule[0])
	}
}

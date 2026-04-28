package view_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/view"
)

// finding builds a single Finding with explicit fields. Helpers below
// compose these into bulk fixtures for invariant tests.
func finding(rule, file string, line int, sev report.Severity, score float64) report.Finding {
	return report.Finding{
		RuleID:   rule,
		File:     file,
		Line:     line,
		Severity: sev,
		Message:  fmt.Sprintf("%s at %s:%d", rule, file, line),
		Score:    score,
	}
}

// findingsAcross builds n findings spread across `rules` distinct rule
// IDs (round-robin) and `pkgs` distinct file directories. Score and
// severity are uniform — vary them at call sites when needed.
func findingsAcross(n, rules, pkgs int, sev report.Severity, score float64) []report.Finding {
	out := make([]report.Finding, n)
	for i := range n {
		rule := fmt.Sprintf("R%d", i%rules)
		pkg := fmt.Sprintf("p%d", i%pkgs)
		out[i] = finding(rule, pkg+"/f.go", i+1, sev, score)
	}
	return out
}

// Invariant: a Leaderboard's Rows must be unique by Label. If multiple
// findings share a RuleID, the picker must aggregate them into one
// row, not emit N visually-identical rows.
func TestInvariant_LeaderboardRowsUniqueByLabel(t *testing.T) {
	// 6 findings, all sharing RuleID "R0": pickLeaderboard
	// fires (top-3 share = 50%) and must produce exactly 1 row OR
	// fall through to a non-Leaderboard view.
	r := report.Report{Findings: findingsAcross(6, 1, 1, report.SeverityWarning, 1)}
	got := view.PickView(r)
	lb, ok := got.(view.Leaderboard)
	if !ok {
		// Falling through to bullet/grouped is also acceptable —
		// a one-row leaderboard is uninformative.
		return
	}
	seen := map[string]int{}
	for _, row := range lb.Rows {
		seen[row.Label]++
	}
	for label, count := range seen {
		if count > 1 {
			t.Fatalf("leaderboard has %d rows labeled %q; want unique labels", count, label)
		}
	}
}

// Invariant: Leaderboard.Total must equal the sum of Row.Value across
// Rows. The bar widget uses Total as the denominator; mismatches
// silently mis-render bar lengths.
func TestInvariant_LeaderboardTotalMatchesRowSum(t *testing.T) {
	cases := []struct {
		name     string
		findings []report.Finding
	}{
		{"6x1rule", findingsAcross(6, 1, 1, report.SeverityWarning, 1)},
		{"9x3rules", findingsAcross(9, 3, 3, report.SeverityWarning, 1)},
		{"varied_scores", []report.Finding{
			finding("a", "p/f.go", 1, report.SeverityError, 5),
			finding("a", "p/f.go", 2, report.SeverityError, 5),
			finding("a", "p/f.go", 3, report.SeverityError, 5),
			finding("b", "p/f.go", 4, report.SeverityWarning, 1),
			finding("c", "p/f.go", 5, report.SeverityWarning, 1),
			finding("d", "p/f.go", 6, report.SeverityWarning, 1),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lb, ok := view.PickView(report.Report{Findings: c.findings}).(view.Leaderboard)
			if !ok {
				return
			}
			var sum float64
			for _, row := range lb.Rows {
				sum += row.Value
			}
			if sum != lb.Total {
				t.Fatalf("Total=%v but rows sum to %v", lb.Total, sum)
			}
		})
	}
}

// Invariant: PickView is a pure function of its input. Calling it
// twice on equal Reports must yield equal ViewSpec values.
// Uses reflect.DeepEqual rather than %#v formatting so map-bearing
// fields (if a future ViewSpec adds one) compare correctly.
func TestInvariant_PickViewDeterministic(t *testing.T) {
	r := report.Report{Findings: findingsAcross(12, 4, 3, report.SeverityWarning, 1)}
	a := view.PickView(r)
	b := view.PickView(r)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("PickView nondeterministic:\n  a=%#v\n  b=%#v", a, b)
	}
}

// Invariant: across the parameter grid, PickView never panics and
// never returns nil. This is a smoke fuzz over the picker surface.
func TestInvariant_PickViewSmokeGrid(t *testing.T) {
	severities := []report.Severity{report.SeverityError, report.SeverityWarning, report.SeverityNote}
	for _, n := range []int{0, 1, 2, 5, 6, 9, 11, 50} {
		for _, rules := range []int{1, 2, 3, 5} {
			for _, pkgs := range []int{1, 2, 4} {
				for _, sev := range severities {
					name := fmt.Sprintf("n=%d/rules=%d/pkgs=%d/sev=%s", n, rules, pkgs, sev)
					t.Run(name, func(t *testing.T) {
						if n == 0 {
							_ = view.PickView(report.Report{})
							return
						}
						r := report.Report{Findings: findingsAcross(n, rules, pkgs, sev, 1)}
						got := view.PickView(r)
						if got == nil {
							t.Fatalf("PickView returned nil for %s", name)
						}
					})
				}
			}
		}
	}
}

// Invariant: when PickView returns a Bullet, the number of items
// equals len(Findings) + len(Tests). No silent drops.
func TestInvariant_BulletPreservesItemCount(t *testing.T) {
	// 9 equal-weight findings → flat distribution → bullet (per
	// existing TestPickView_Leaderboard_FlatDistribution_FallsThrough).
	findings := findingsAcross(9, 1, 1, report.SeverityWarning, 1)
	r := report.Report{Findings: findings}
	got, ok := view.PickView(r).(view.Bullet)
	if !ok {
		t.Fatalf("want Bullet for flat distribution, got %T", view.PickView(r))
	}
	if len(got.Items) != len(findings) {
		t.Fatalf("Bullet has %d items; want %d", len(got.Items), len(findings))
	}
}

// Invariant: when PickView returns a Grouped, the sum of items
// across sections equals len(Findings).
func TestInvariant_GroupedPreservesItemCount(t *testing.T) {
	// > 10 findings triggers Grouped when SmallMultiples doesn't fit.
	// Use 11 findings across 1 package so SmallMultiples is rejected.
	findings := findingsAcross(11, 1, 1, report.SeverityWarning, 1)
	got, ok := view.PickView(report.Report{Findings: findings}).(view.Grouped)
	if !ok {
		t.Skipf("picker chose %T not Grouped — invariant not applicable", view.PickView(report.Report{Findings: findings}))
	}
	var total int
	for _, s := range got.Sections {
		total += len(s.Items)
	}
	if total != len(findings) {
		t.Fatalf("Grouped has %d items across sections; want %d", total, len(findings))
	}
}

// Sanity: with the aggregating picker, a moderately diverse mix
// (10 findings across 5 rule IDs, score 1 each) should still fall
// through to a non-Leaderboard view. If this starts returning
// Leaderboard the threshold needs re-tuning.
func TestInvariant_LeaderboardThresholdSane(t *testing.T) {
	r := report.Report{Findings: findingsAcross(10, 5, 1, report.SeverityWarning, 1)}
	got := view.PickView(r)
	if _, ok := got.(view.Leaderboard); ok {
		t.Fatalf("threshold may need re-tuning post-aggregation: got Leaderboard for 10/5 diverse mix; want fall-through. Picked %T", got)
	}
}

package view_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/view"
)

func mkFindings(n int, sev report.Severity, pkg string) []report.Finding {
	out := make([]report.Finding, n)
	for i := 0; i < n; i++ {
		out[i] = report.Finding{
			RuleID:   "R",
			File:     pkg + "/f.go",
			Line:     i + 1,
			Severity: sev,
			Message:  "msg",
			Score:    1,
		}
	}
	return out
}

func TestPickView_Clean_EmptyReport(t *testing.T) {
	got := view.PickView(report.Report{})
	if _, ok := got.(view.Clean); !ok {
		t.Fatalf("want Clean, got %T", got)
	}
}

func TestPickView_Clean_AllPass(t *testing.T) {
	r := report.Report{Tests: []report.TestResult{
		{Package: "p", Test: "T1", Outcome: report.OutcomePass},
		{Package: "p", Test: "T2", Outcome: report.OutcomeSkip},
	}}
	if _, ok := view.PickView(r).(view.Clean); !ok {
		t.Fatalf("want Clean")
	}
}

func TestPickView_Headline_Panic(t *testing.T) {
	r := report.Report{Tests: []report.TestResult{
		{Package: "p", Test: "T1", Outcome: report.OutcomePanic},
	}}
	h, ok := view.PickView(r).(view.Headline)
	if !ok {
		t.Fatalf("want Headline, got %T", view.PickView(r))
	}
	if h.Title != "PANIC" {
		t.Errorf("title = %q", h.Title)
	}
}

func TestPickView_Headline_BuildErrorOnly(t *testing.T) {
	r := report.Report{Tests: []report.TestResult{
		{Package: "p", Outcome: report.OutcomeBuildError},
		{Package: "q", Outcome: report.OutcomePass},
	}}
	if _, ok := view.PickView(r).(view.Headline); !ok {
		t.Fatalf("want Headline")
	}
}

func TestPickView_Headline_BuildErrorMixedWithFail_NotHeadline(t *testing.T) {
	r := report.Report{Tests: []report.TestResult{
		{Package: "p", Outcome: report.OutcomeBuildError},
		{Package: "q", Test: "T", Outcome: report.OutcomeFail},
	}}
	if _, ok := view.PickView(r).(view.Headline); ok {
		t.Fatalf("did not want Headline")
	}
}

func TestPickView_Alert_SingleFinding(t *testing.T) {
	r := report.Report{Findings: mkFindings(1, report.SeverityError, "a")}
	a, ok := view.PickView(r).(view.Alert)
	if !ok {
		t.Fatalf("want Alert, got %T", view.PickView(r))
	}
	if a.Severity != report.SeverityError {
		t.Errorf("severity = %v", a.Severity)
	}
}

func TestPickView_Bullet_TwoFindings(t *testing.T) {
	r := report.Report{Findings: mkFindings(2, report.SeverityWarning, "a")}
	if _, ok := view.PickView(r).(view.Bullet); !ok {
		t.Fatalf("want Bullet, got %T", view.PickView(r))
	}
}

func TestPickView_Leaderboard_TopThreeDominant(t *testing.T) {
	// 6 findings; first three Score=10, rest Score=1 → head = 30/33 > 50%.
	fs := mkFindings(6, report.SeverityWarning, "a")
	fs[0].Score, fs[1].Score, fs[2].Score = 10, 10, 10
	fs[3].Score, fs[4].Score, fs[5].Score = 1, 1, 1
	r := report.Report{Findings: fs}
	if _, ok := view.PickView(r).(view.Leaderboard); !ok {
		t.Fatalf("want Leaderboard, got %T", view.PickView(r))
	}
}

func TestPickView_Leaderboard_BelowMinTotal(t *testing.T) {
	// 5 findings — under min total threshold of 6.
	r := report.Report{Findings: mkFindings(5, report.SeverityWarning, "a")}
	if _, ok := view.PickView(r).(view.Leaderboard); ok {
		t.Fatalf("did not want Leaderboard at count=5")
	}
}

func TestPickView_Leaderboard_FlatDistribution_FallsThrough(t *testing.T) {
	// 6 equal scores → head share = 3/6 = 50.0% — exactly at threshold,
	// passes. So make 9 equal to drop head share to 33%.
	r := report.Report{Findings: mkFindings(9, report.SeverityWarning, "a")}
	got := view.PickView(r)
	if _, ok := got.(view.Leaderboard); ok {
		t.Fatalf("did not want Leaderboard for flat distribution, got %T", got)
	}
}

func TestPickView_SmallMultiples_ThreeGroups(t *testing.T) {
	// 3 groups × 3 items = 9 equal-weight findings → Leaderboard head
	// share is 3/9 = 33%, falling through to SmallMultiples.
	fs := make([]report.Finding, 0, 9)
	fs = append(fs, mkFindings(3, report.SeverityWarning, "a")...)
	fs = append(fs, mkFindings(3, report.SeverityWarning, "b")...)
	fs = append(fs, mkFindings(3, report.SeverityWarning, "c")...)
	r := report.Report{Findings: fs}
	if _, ok := view.PickView(r).(view.SmallMultiples); !ok {
		t.Fatalf("want SmallMultiples, got %T", view.PickView(r))
	}
}

func TestPickView_SmallMultiples_GroupTooSmall_FallsThrough(t *testing.T) {
	fs := make([]report.Finding, 0, 5)
	fs = append(fs, mkFindings(2, report.SeverityWarning, "a")...)
	fs = append(fs, mkFindings(2, report.SeverityWarning, "b")...)
	fs = append(fs, mkFindings(1, report.SeverityWarning, "c")...) // only 1
	r := report.Report{Findings: fs}
	if _, ok := view.PickView(r).(view.SmallMultiples); ok {
		t.Fatalf("did not want SmallMultiples when a group has < 2")
	}
}

func TestPickView_Grouped_LargeFlat(t *testing.T) {
	// 11 findings, single package → fails SmallMultiples, fails Leaderboard
	// (flat scores), passes Grouped (count > 10).
	r := report.Report{Findings: mkFindings(11, report.SeverityWarning, "a")}
	got := view.PickView(r)
	if _, ok := got.(view.Grouped); !ok {
		t.Fatalf("want Grouped, got %T", got)
	}
}

func TestPickView_Grouped_Boundary(t *testing.T) {
	// 10 findings — at the boundary, should NOT be Grouped (threshold > 10).
	r := report.Report{Findings: mkFindings(10, report.SeverityWarning, "a")}
	if _, ok := view.PickView(r).(view.Grouped); ok {
		t.Fatalf("did not want Grouped at count=10")
	}
}

func TestPickView_Bullet_DefaultFallback(t *testing.T) {
	// 4 findings, single package — none of the higher-priority branches
	// fire, so we land on Bullet.
	r := report.Report{Findings: mkFindings(4, report.SeverityWarning, "a")}
	if _, ok := view.PickView(r).(view.Bullet); !ok {
		t.Fatalf("want Bullet, got %T", view.PickView(r))
	}
}

func TestPickView_Delta_WrapsInner(t *testing.T) {
	r := report.Report{
		Findings: mkFindings(2, report.SeverityWarning, "a"),
		Diff: &report.DiffSummary{
			New: []report.DiffItem{
				{Severity: string(report.SeverityWarning)},
				{Severity: string(report.SeverityWarning)},
			},
		},
	}
	d, ok := view.PickView(r).(view.Delta)
	if !ok {
		t.Fatalf("want Delta, got %T", view.PickView(r))
	}
	if _, ok := d.Inner.(view.Bullet); !ok {
		t.Fatalf("want Bullet inner, got %T", d.Inner)
	}
}

func TestPickView_Delta_NoChange_NoWrap(t *testing.T) {
	r := report.Report{
		Findings: mkFindings(2, report.SeverityWarning, "a"),
		Diff:     &report.DiffSummary{PersistentCount: 2},
	}
	if _, ok := view.PickView(r).(view.Delta); ok {
		t.Fatalf("did not want Delta when buckets are identical")
	}
}

func TestPickView_Determinism(t *testing.T) {
	r := report.Report{Findings: mkFindings(7, report.SeverityWarning, "a")}
	a := view.PickView(r)
	b := view.PickView(r)
	if got, want := variantName(a), variantName(b); got != want {
		t.Fatalf("variant differs across calls: %s vs %s", got, want)
	}
}

func variantName(v view.ViewSpec) string {
	switch v.(type) {
	case view.Clean:
		return "Clean"
	case view.Bullet:
		return "Bullet"
	case view.Grouped:
		return "Grouped"
	case view.Leaderboard:
		return "Leaderboard"
	case view.Headline:
		return "Headline"
	case view.Alert:
		return "Alert"
	case view.Delta:
		return "Delta"
	case view.SmallMultiples:
		return "SmallMultiples"
	default:
		return "?"
	}
}

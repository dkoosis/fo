package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
)

// Thresholds locked at fo-7f5 design pass; tuning happens against real
// fixtures, not by editing renderers.
const (
	leaderboardMinTotal = 6
	// leaderboardHeadShare: post-aggregation distributions concentrate;
	// threshold tuned 2026-04-27 against fo-abv invariants.
	leaderboardHeadShare   = 0.70
	smallMultiplesMinGroup = 3
	smallMultiplesMinItems = 2
	groupedMinCount        = 10
	alertErrorThreshold    = 1
)

// Mode hints the audience for the picked view. ModeHuman (default) may
// pick visual aggregates (Leaderboard, SmallMultiples) that condense
// many findings into a chart. ModeLLM skips those — an LLM consumer
// needs file:line per finding, not a bar chart — and falls through to
// Grouped or Bullet, which carry the full data.
type Mode int

const (
	ModeHuman Mode = iota
	ModeLLM
)

// PickView selects a ViewSpec from a Report. Pure and deterministic:
// branches are evaluated in fixed priority order so the same Report
// always yields the same shape. Delta wraps the inner pick when Diff
// is present and at least one severity bucket moved.
func PickView(r report.Report) ViewSpec {
	return PickViewMode(r, ModeHuman)
}

// PickViewMode is PickView with an explicit audience mode.
func PickViewMode(r report.Report, mode Mode) ViewSpec {
	inner := pickInner(r, mode)
	if r.Diff != nil {
		buckets := deltaBuckets(r, r.Diff)
		if hasNonZero(buckets) {
			return Delta{Inner: inner, Buckets: buckets, Headline: r.Diff.Headline}
		}
	}
	return inner
}

func pickInner(r report.Report, mode Mode) ViewSpec {
	if isClean(r) {
		return Clean{Message: "no findings"}
	}
	if h, ok := pickHeadline(r); ok {
		return h
	}
	if a, ok := pickAlert(r); ok {
		return a
	}
	if mode != ModeLLM {
		if lb, ok := pickLeaderboard(r); ok {
			return lb
		}
		if sm, ok := pickSmallMultiples(r); ok {
			return sm
		}
	}
	if g, ok := pickGrouped(r); ok {
		return g
	}
	return pickBullet(r)
}

func isClean(r report.Report) bool {
	if len(r.Findings) > 0 {
		return false
	}
	for _, t := range r.Tests {
		if t.Outcome != report.OutcomePass && t.Outcome != report.OutcomeSkip {
			return false
		}
	}
	return true
}

func pickHeadline(r report.Report) (Headline, bool) {
	for _, t := range r.Tests {
		if t.Outcome == report.OutcomePanic {
			return Headline{
				Title:  "PANIC",
				Detail: panicDetail(t),
				Body:   panicBody(t.Output),
			}, true
		}
	}
	if buildErrorOnly(r) {
		t := r.Tests[0]
		return Headline{
			Title:  "BUILD FAILED",
			Detail: t.Package,
			Body:   buildErrorBody(t.Output),
		}, true
	}
	return Headline{}, false
}

func panicDetail(t report.TestResult) string {
	if t.Test != "" {
		return fmt.Sprintf("%s.%s", t.Package, t.Test)
	}
	return t.Package
}

// panicBody extracts the panic message and the first user-code stack
// frame from raw test output. Frames in /testing/, /runtime/, or
// belonging to the testing/runtime packages are skipped so the LLM
// reader sees the user's call site instead of harness scaffolding.
func panicBody(output string) []string {
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	var msg, frame string
	for i, ln := range lines {
		trim := strings.TrimSpace(ln)
		if msg == "" && strings.HasPrefix(trim, "panic:") {
			msg = trim
			continue
		}
		if frame == "" && msg != "" && isUserFrame(trim) && i+1 < len(lines) {
			loc := strings.TrimSpace(lines[i+1])
			loc = stripFramePC(loc)
			if loc != "" {
				frame = loc
			}
		}
	}
	out := make([]string, 0, 2)
	if msg != "" {
		out = append(out, msg)
	}
	if frame != "" {
		out = append(out, "at "+frame)
	}
	return out
}

func isUserFrame(funcLine string) bool {
	if funcLine == "" || !strings.Contains(funcLine, "(") {
		return false
	}
	switch {
	case strings.HasPrefix(funcLine, "testing."),
		strings.HasPrefix(funcLine, "runtime."),
		strings.HasPrefix(funcLine, "panic("),
		strings.HasPrefix(funcLine, "created by testing"):
		return false
	}
	return true
}

// stripFramePC removes the trailing " +0x..." program counter from a
// stack-frame source line so the LLM-facing output stays stable across
// builds. "src/foo.go:12 +0x2c" → "src/foo.go:12".
func stripFramePC(s string) string {
	if i := strings.LastIndex(s, " +0x"); i > 0 {
		return s[:i]
	}
	return s
}

// buildErrorBody returns the first non-empty line of build output —
// typically "file:line:col: msg", which is what the user needs to fix.
func buildErrorBody(output string) []string {
	for ln := range strings.SplitSeq(output, "\n") {
		trim := strings.TrimSpace(ln)
		if trim == "" || strings.HasPrefix(trim, "# ") {
			continue
		}
		return []string{trim}
	}
	return nil
}

// buildErrorOnly: at least one build_error and no other failure modes.
func buildErrorOnly(r report.Report) bool {
	if len(r.Findings) > 0 || len(r.Tests) == 0 {
		return false
	}
	sawBuild := false
	for _, t := range r.Tests {
		switch t.Outcome {
		case report.OutcomeBuildError:
			sawBuild = true
		case report.OutcomeFail, report.OutcomePanic:
			return false
		case report.OutcomePass, report.OutcomeSkip:
			// not a build-error-only condition
		}
	}
	return sawBuild
}

func pickAlert(r report.Report) (Alert, bool) {
	// Single-error breach — one finding deserves a one-line alert
	// rather than a list. Uses count==1 to avoid stealing from Bullet.
	if len(r.Findings) == alertErrorThreshold && len(r.Tests) == 0 {
		f := r.Findings[0]
		return Alert{
			Severity: f.Severity,
			Prefix:   alertPrefix(f.Severity),
			Value:    f.Message,
			Detail:   fmt.Sprintf("%s:%d", f.File, f.Line),
		}, true
	}
	return Alert{}, false
}

func alertPrefix(s report.Severity) string {
	switch s {
	case report.SeverityError:
		return "ERROR"
	case report.SeverityWarning:
		return "WARNING"
	case report.SeverityNote:
		return "NOTE"
	default:
		return "NOTE"
	}
}

func pickLeaderboard(r report.Report) (Leaderboard, bool) {
	if len(r.Findings) < leaderboardMinTotal {
		return Leaderboard{}, false
	}

	// Aggregate by label — one row per distinct label, summing scores.
	// Findings with the same RuleID would otherwise emit N visually
	// identical rows (see fo-abv).
	agg := map[string]float64{}
	order := []string{}
	var total float64
	for _, f := range r.Findings {
		v := f.Score
		if v <= 0 {
			v = 1
		}
		label := lbLabel(f)
		if _, seen := agg[label]; !seen {
			order = append(order, label)
		}
		agg[label] += v
		total += v
	}

	// A one-row leaderboard says nothing a bullet can't say better.
	if len(agg) < 2 {
		return Leaderboard{}, false
	}

	rows := make([]LbRow, 0, len(agg))
	for _, label := range order {
		rows = append(rows, LbRow{Label: label, Value: agg[label]})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Value > rows[j].Value })

	head := min(3, len(rows))
	var headSum float64
	for i := range head {
		headSum += rows[i].Value
	}
	if total == 0 || headSum/total < leaderboardHeadShare {
		return Leaderboard{}, false
	}
	return Leaderboard{Rows: rows, Total: total}, true
}

func lbLabel(f report.Finding) string {
	if f.RuleID != "" {
		return f.RuleID
	}
	return f.Message
}

func pickSmallMultiples(r report.Report) (SmallMultiples, bool) {
	groups := groupFindingsByPackage(r.Findings)
	if len(groups) < smallMultiplesMinGroup {
		return SmallMultiples{}, false
	}
	for _, items := range groups {
		if len(items) < smallMultiplesMinItems {
			return SmallMultiples{}, false
		}
	}
	keys := sortedKeys(groups)
	cells := make([]MultipleCell, 0, len(keys))
	for _, k := range keys {
		cells = append(cells, MultipleCell{
			Label:    k,
			Counters: severityCounters(groups[k]),
		})
	}
	return SmallMultiples{Cells: cells}, true
}

func groupFindingsByPackage(fs []report.Finding) map[string][]report.Finding {
	out := make(map[string][]report.Finding)
	for _, f := range fs {
		key := packageOf(f.File)
		out[key] = append(out[key], f)
	}
	return out
}

// packageOf reduces a file path to its directory, our proxy for
// "package" without parsing Go source.
func packageOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func sortedKeys(m map[string][]report.Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func severityCounters(fs []report.Finding) []Counter {
	var e, w, n int
	for _, f := range fs {
		switch f.Severity {
		case report.SeverityError:
			e++
		case report.SeverityWarning:
			w++
		case report.SeverityNote:
			n++
		}
	}
	out := make([]Counter, 0, 3)
	if e > 0 {
		out = append(out, Counter{Severity: report.SeverityError, Label: "err", Value: e})
	}
	if w > 0 {
		out = append(out, Counter{Severity: report.SeverityWarning, Label: "warn", Value: w})
	}
	if n > 0 {
		out = append(out, Counter{Severity: report.SeverityNote, Label: "note", Value: n})
	}
	return out
}

func pickGrouped(r report.Report) (Grouped, bool) {
	if len(r.Findings) <= groupedMinCount {
		return Grouped{}, false
	}
	bySev := map[report.Severity][]BulletItem{}
	for _, f := range r.Findings {
		bySev[f.Severity] = append(bySev[f.Severity], findingItem(f))
	}
	order := []report.Severity{report.SeverityError, report.SeverityWarning, report.SeverityNote}
	sections := make([]GroupedSection, 0, len(order))
	for _, s := range order {
		if items := bySev[s]; len(items) > 0 {
			sections = append(sections, GroupedSection{Label: string(s), Items: items})
		}
	}
	return Grouped{Sections: sections}, true
}

func pickBullet(r report.Report) Bullet {
	items := make([]BulletItem, 0, len(r.Findings)+len(r.Tests))
	for _, f := range r.Findings {
		items = append(items, findingItem(f))
	}
	for _, t := range r.Tests {
		items = append(items, testItem(t))
	}
	return Bullet{Items: items}
}

func findingItem(f report.Finding) BulletItem {
	return BulletItem{
		Severity:   f.Severity,
		Label:      f.Message,
		Value:      fmt.Sprintf("%s:%d", f.File, f.Line),
		FixCommand: f.FixCommand,
	}
}

func testItem(t report.TestResult) BulletItem {
	label := t.Test
	if label == "" {
		label = t.Package
	}
	return BulletItem{
		Outcome:    t.Outcome,
		Label:      label,
		Value:      t.Package,
		FixCommand: t.FixCommand,
	}
}

// deltaBuckets summarises change vs prior across the standard buckets.
// Direction is derived from Diff classification (New/Resolved/Regressed
// per severity); the fail bucket is always 0-direction because state
// does not persist test outcomes — the bucket stays for layout symmetry.
func deltaBuckets(cur report.Report, d *report.DiffSummary) []DeltaBucket {
	curE, curW, curN := severityCounts(cur.Findings)
	curF := failCount(cur.Tests)
	dE := severityDelta(d, string(report.SeverityError))
	dW := severityDelta(d, string(report.SeverityWarning))
	dN := severityDelta(d, string(report.SeverityNote))
	return []DeltaBucket{
		{Label: "err", Count: curE, Direction: sign(dE)},
		{Label: "warn", Count: curW, Direction: sign(dW)},
		{Label: "note", Count: curN, Direction: sign(dN)},
		{Label: "fail", Count: curF, Direction: 0},
	}
}

func severityDelta(d *report.DiffSummary, sev string) int {
	delta := 0
	for _, it := range d.New {
		if it.Severity == sev {
			delta++
		}
	}
	for _, it := range d.Resolved {
		if it.Severity == sev {
			delta--
		}
	}
	for _, it := range d.Regressed {
		if it.Severity == sev {
			delta++
		}
		if it.PriorSeverity == sev {
			delta--
		}
	}
	return delta
}

func severityCounts(fs []report.Finding) (int, int, int) {
	var e, w, n int
	for _, f := range fs {
		switch f.Severity {
		case report.SeverityError:
			e++
		case report.SeverityWarning:
			w++
		case report.SeverityNote:
			n++
		}
	}
	return e, w, n
}

func failCount(ts []report.TestResult) int {
	var c int
	for _, t := range ts {
		if t.Outcome == report.OutcomeFail || t.Outcome == report.OutcomePanic || t.Outcome == report.OutcomeBuildError {
			c++
		}
	}
	return c
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

func hasNonZero(bs []DeltaBucket) bool {
	for _, b := range bs {
		if b.Direction != 0 {
			return true
		}
	}
	return false
}

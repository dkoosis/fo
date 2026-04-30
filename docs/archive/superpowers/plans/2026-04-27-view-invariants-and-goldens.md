# View Invariants & Real-Pipeline Goldens Implementation Plan

> **For agentic workers:** Use dk:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Catch logic bugs in `pkg/view` (the report → ViewSpec picker and renderers) before they reach users, by adding (a) invariant tests over `PickView` outputs and (b) end-to-end golden fixtures captured from real tool pipelines. Also fixes the first bug this approach surfaces: `fo-abv` (leaderboard rows not aggregated by label).

**Architecture:**
- **Layer 1 — Invariant tests** (`pkg/view/invariants_test.go`): table-driven cases that vary finding count × rule-ID cardinality × severity mix × score distribution, then assert structural properties of the picked `ViewSpec` (e.g. "no two leaderboard rows share a label", "leaderboard total == sum of row values"). Pure stdlib `testing` — no new deps. These would have caught `fo-abv` on day one.
- **Layer 2 — Pipeline goldens** (`pkg/view/testdata/pipelines/`): captured stdin streams from real tools (jscpd-only, vet-only, lint-only, mixed, clean, panic, build-error). A test runs each through `fo --format human/llm/json` and diffs against committed `.golden` files. Refresh with `go test ./pkg/view/... -update` (matching the existing `-update` flag in `pkg/view/view_test.go:18`).
- **Layer 3 — Bug fix:** With Layer 1 in place, fix `pickLeaderboard` to aggregate by label and fall through when only one distinct label exists.

**Tech Stack:** Go 1.24, stdlib `testing` only (project policy: deps are lipgloss + x/term). Golden refresh uses the `-update` flag declared in `pkg/view/view_test.go:18` — the new pipeline test reuses that same flag (both files are in package `view_test`).

**Scope notes:**
- Picker and renderers only. SARIF parsing, testjson parsing, mapper, and wrappers are out of scope — they have separate test surfaces.
- Goldens are committed text files; this plan does not introduce snapshot frameworks.
- Refreshing goldens after a deliberate format change is a one-command operation (`go test ./pkg/view/... -update`).

**Beads:** filing/closing happens in Task 0 and Task 7. The leaderboard fix closes `fo-abv`. A new bead tracks the invariants harness itself for future reference.

---

## File Structure

**Create:**
- `pkg/view/invariants_test.go` — table-driven invariant tests over `PickView`.
- `pkg/view/pipeline_golden_test.go` — runs captured stdin streams through the full `cmd/fo` binary and diffs output.
- `pkg/view/testdata/pipelines/jscpd_only.in` — captured jscpd-only SARIF stream (Layer 2 fixture).
- `pkg/view/testdata/pipelines/vet_only.in` — captured `go vet` diag stream.
- `pkg/view/testdata/pipelines/mixed.in` — captured multi-tool stream (vet + jscpd + test-json).
- `pkg/view/testdata/pipelines/clean.in` — captured all-pass stream.
- `pkg/view/testdata/pipelines/panic.in` — captured panicking-test stream.
- `pkg/view/testdata/pipelines/build_error.in` — captured build-error stream.
- `pkg/view/testdata/pipelines/<name>.human.golden` and `<name>.llm.golden` — one pair per `.in` file.

**Modify:**
- `pkg/view/pickview.go:136-161` — fix `pickLeaderboard` aggregation (Task 6).
- `Makefile` — add `qa-goldens` target that runs Layer 2 against a freshly-built `fo` (Task 8).

---

## Task 0: Set up beads & branch

- [ ] **Step 1: Create branch**

```bash
git checkout -b qa/view-invariants-goldens
```

- [ ] **Step 2: File a tracking bead for the invariants harness**

```bash
bd create \
  --title="QA: invariants + pipeline goldens for pkg/view" \
  --description="Add table-driven invariant tests over PickView (no two leaderboard rows share a label, total == sum of row values, deterministic, etc.) and golden fixtures captured from real tool pipelines (jscpd-only, vet-only, mixed, clean, panic, build-error). Layer 1 prevents picker logic regressions; Layer 2 prevents end-to-end format regressions. Plan: docs/superpowers/plans/2026-04-27-view-invariants-and-goldens.md" \
  --type=task --priority=2
```

Record the returned bead ID — referred to below as `<harness-bead>`.

- [ ] **Step 3: Confirm fo-abv is open and unclaimed**

Run: `bd show fo-abv`
Expected: status `open`. If anything else, **stop** and surface the discrepancy to the human — do not proceed.

- [ ] **Step 4: Claim both**

```bash
bd update <harness-bead> --claim
bd update fo-abv --claim
```

---

## Task 1: Invariant test scaffold

**Files:**
- Create: `pkg/view/invariants_test.go`

- [ ] **Step 1: Create the file with shared helpers**

```go
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
	for i := 0; i < n; i++ {
		rule := fmt.Sprintf("R%d", i%rules)
		pkg := fmt.Sprintf("p%d", i%pkgs)
		out[i] = finding(rule, pkg+"/f.go", i+1, sev, score)
	}
	return out
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./pkg/view/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add pkg/view/invariants_test.go
git commit -m "test: scaffold view invariant test helpers"
```

---

## Task 2: Invariant — leaderboard rows are unique by label

**Files:**
- Modify: `pkg/view/invariants_test.go`

This is the test that fails today and exposes `fo-abv`.

- [ ] **Step 1: Add the failing test**

Append to `pkg/view/invariants_test.go`:

```go
// Invariant: a Leaderboard's Rows must be unique by Label. If multiple
// findings share a RuleID, the picker must aggregate them into one
// row, not emit N visually-identical rows.
func TestInvariant_LeaderboardRowsUniqueByLabel(t *testing.T) {
	// 6 findings, all sharing RuleID "code-clone": pickLeaderboard
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
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./pkg/view/ -run TestInvariant_LeaderboardRowsUniqueByLabel -v`
Expected: FAIL with `leaderboard has 6 rows labeled "R0"; want unique labels`.

- [ ] **Step 3: Commit the failing test (TDD red)**

```bash
git add pkg/view/invariants_test.go
git commit -m "test: invariant — leaderboard rows unique by label (red, exposes fo-abv)"
```

---

## Task 3: Invariant — leaderboard total equals sum of row values

**Files:**
- Modify: `pkg/view/invariants_test.go`

- [ ] **Step 1: Add the test**

Append:

```go
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
```

- [ ] **Step 2: Run and observe**

Run: `go test ./pkg/view/ -run TestInvariant_LeaderboardTotalMatchesRowSum -v`
Expected today: PASS (the existing code happens to satisfy this — but we lock it in before refactoring).

- [ ] **Step 3: Commit**

```bash
git add pkg/view/invariants_test.go
git commit -m "test: invariant — leaderboard total equals sum of row values"
```

---

## Task 4: Invariant — picker is deterministic & total-preserving

**Files:**
- Modify: `pkg/view/invariants_test.go`

- [ ] **Step 1: Add tests**

Append:

```go
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
```

- [ ] **Step 2: Run**

Run: `go test ./pkg/view/ -run TestInvariant_PickView -v`
Expected: all subtests PASS.

- [ ] **Step 3: Commit**

```bash
git add pkg/view/invariants_test.go
git commit -m "test: invariant — picker determinism and smoke grid"
```

---

## Task 5: Invariant — Bullet/Grouped preserve all findings

**Files:**
- Modify: `pkg/view/invariants_test.go`

- [ ] **Step 1: Add tests**

Append:

```go
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
```

- [ ] **Step 2: Run**

Run: `go test ./pkg/view/ -run TestInvariant_ -v`
Expected: all PASS *except* `TestInvariant_LeaderboardRowsUniqueByLabel` from Task 2, which still fails.

- [ ] **Step 3: Commit**

```bash
git add pkg/view/invariants_test.go
git commit -m "test: invariant — Bullet/Grouped preserve all findings"
```

---

## Task 6: Fix `pickLeaderboard` (closes fo-abv)

**Files:**
- Modify: `pkg/view/pickview.go:136-161`

- [ ] **Step 1: Read the current implementation**

Run: `sed -n '136,161p' pkg/view/pickview.go`
Confirm the function matches the structure described in the plan header (one `LbRow` per `Finding`, `Label = lbLabel(f)`).

- [ ] **Step 2: Replace `pickLeaderboard` with the aggregating version**

Edit `pkg/view/pickview.go`. Find:

```go
func pickLeaderboard(r report.Report) (Leaderboard, bool) {
	if len(r.Findings) < leaderboardMinTotal {
		return Leaderboard{}, false
	}
	rows := make([]LbRow, 0, len(r.Findings))
	var total float64
	for _, f := range r.Findings {
		v := f.Score
		if v <= 0 {
			v = 1
		}
		rows = append(rows, LbRow{Label: lbLabel(f), Value: v})
		total += v
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
```

Replace with:

```go
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
```

- [ ] **Step 3: Run the invariant suite**

Run: `go test ./pkg/view/ -run TestInvariant_ -v`
Expected: all subtests PASS, including `TestInvariant_LeaderboardRowsUniqueByLabel`.

- [ ] **Step 4: Update the existing leaderboard test to use distinct rule IDs**

`pkg/view/pickview_test.go:9-22` `mkFindings` hardcodes `RuleID: "R"`. After the fix, `TestPickView_Leaderboard_TopThreeDominant` (line 93) aggregates to one row and falls through. The dominance-test intent is still valid — it just needs distinct rule IDs to exercise the multi-row path.

Add this helper to `pkg/view/pickview_test.go` (immediately after `mkFindings`):

```go
// mkFindingsDistinct builds n findings each with a unique RuleID
// ("R0", "R1", ...). Use when a test must exercise the multi-row
// leaderboard path post-aggregation.
func mkFindingsDistinct(n int, sev report.Severity, pkg string) []report.Finding {
	out := make([]report.Finding, n)
	for i := range n {
		out[i] = report.Finding{
			RuleID:   fmt.Sprintf("R%d", i),
			File:     pkg + "/f.go",
			Line:     i + 1,
			Severity: sev,
			Message:  "msg",
			Score:    1,
		}
	}
	return out
}
```

If `pickview_test.go` does not already import `"fmt"`, add it.

Then replace the body of `TestPickView_Leaderboard_TopThreeDominant`:

```go
func TestPickView_Leaderboard_TopThreeDominant(t *testing.T) {
	// 6 findings, distinct rule IDs; first three Score=10, rest Score=1
	// → head = 30/33 > 50%. Distinct labels keep aggregation a no-op.
	fs := mkFindingsDistinct(6, report.SeverityWarning, "a")
	fs[0].Score, fs[1].Score, fs[2].Score = 10, 10, 10
	fs[3].Score, fs[4].Score, fs[5].Score = 1, 1, 1
	r := report.Report{Findings: fs}
	if _, ok := view.PickView(r).(view.Leaderboard); !ok {
		t.Fatalf("want Leaderboard, got %T", view.PickView(r))
	}
}
```

`TestPickView_Leaderboard_FlatDistribution_FallsThrough` (line 112) is fine as-is post-fix: 9 findings with shared `RuleID="R"` aggregate to one row and fall through (now via the `len(agg) < 2` short-circuit rather than the head-share check). The original assertion ("did not want Leaderboard") still holds. **Do not change it.**

- [ ] **Step 5: Run the existing `pkg/view` suite**

Run: `go test ./pkg/view/...`
Expected: all PASS — including the modified `TestPickView_Leaderboard_TopThreeDominant`. If anything else fails, stop and investigate.

- [ ] **Step 6: Sanity-check the head-share threshold**

The 0.50 threshold (`leaderboardHeadShare`, `pickview.go:14`) was tuned against per-finding rows; aggregation skews distributions toward concentration (one big rule sums up). This is not necessarily wrong, but verify it explicitly. Add this test to `pkg/view/invariants_test.go`:

```go
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
```

Run: `go test ./pkg/view/ -run TestInvariant_LeaderboardThresholdSane -v`
Expected: PASS. If it fails, **stop**: surface to the human — the threshold needs re-tuning and that is a design decision, not an execution step.

- [ ] **Step 7: Refresh affected goldens**

Run: `go test ./pkg/view/ -update`
Then: `git diff pkg/view/testdata/golden/`
Expected: `leaderboard_top3.golden` (and possibly others) update. Every row label in the new golden should be unique. If anything outside `leaderboard_top3.golden` changes, **stop** and investigate before committing.

- [ ] **Step 8: Repro the original trixi bug**

```bash
go build -o /tmp/fo-fixed ./cmd/fo
cd "$HOME/Projects/trixi" 2>/dev/null || cd "$HOME/projects/trixi"
TMP=$(mktemp -d)
jscpd . --silent --reporters json --output "$TMP" >/dev/null 2>&1
{
  echo '--- tool:dupl format:sarif ---'
  cat "$TMP/jscpd-report.json" | /tmp/fo-fixed wrap jscpd
  echo
} | /tmp/fo-fixed --format human
rm -rf "$TMP"
```
Expected: at most one row per distinct rule ID (or a bullet list) — not N identical `code-clone` rows.

- [ ] **Step 9: Commit**

```bash
git add pkg/view/pickview.go pkg/view/pickview_test.go pkg/view/invariants_test.go pkg/view/testdata/golden/
git commit -m "fix(view): aggregate leaderboard rows by label (closes fo-abv)

When N findings shared a RuleID, pickLeaderboard emitted N
visually-identical rows. Now aggregates by label, sums scores, and
falls through when only one distinct label remains."
```

- [ ] **Step 10: Close the bead**

```bash
bd close fo-abv --reason="aggregate by label + single-label fall-through; tests in pkg/view/invariants_test.go"
```

---

## Task 7: Capture pipeline fixtures

**Files:**
- Create: `pkg/view/testdata/pipelines/{jscpd_only,vet_only,mixed,clean,panic,build_error}.in`

These are real stdin streams. Capture once; commit as text. They become the inputs Layer 2 replays.

- [ ] **Step 1: Create the directory**

```bash
mkdir -p pkg/view/testdata/pipelines
```

All fixtures must be capturable **offline** with no module-cache dependencies. Where the original tool needs network or a fresh module init, capture the tool's text output once into a static input file and feed that through `fo wrap …` (which is offline).

- [ ] **Step 2: Capture `jscpd_only.in`**

Use trixi as the source repo (it has known duplication). Note: trixi may live at `$HOME/Projects/trixi` or `$HOME/projects/trixi` depending on the host; the snippet probes both.

```bash
TRIXI=$HOME/Projects/trixi
[ -d "$TRIXI" ] || TRIXI=$HOME/projects/trixi
[ -d "$TRIXI" ] || { echo "trixi not found; capture jscpd_only.in from any repo with duplication"; exit 1; }

TMP=$(mktemp -d)
( cd "$TRIXI" && jscpd . --silent --reporters json --output "$TMP" >/dev/null 2>&1 )
{
  echo '--- tool:dupl format:sarif ---'
  cat "$TMP/jscpd-report.json" | go run ./cmd/fo wrap jscpd
  echo
} > pkg/view/testdata/pipelines/jscpd_only.in
rm -rf "$TMP"
```

Verify: `head -3 pkg/view/testdata/pipelines/jscpd_only.in` shows the delimiter line, then SARIF JSON.

- [ ] **Step 3: Capture `vet_only.in` (offline, canned input)**

Skip invoking `go vet` (it needs a module cache and can hit the network). Instead, hand-craft a govet-style diag-text input and pipe it through `wrap diag`, which is the same path the multiplexer uses:

```bash
{
  echo '--- tool:vet format:sarif ---'
  printf '%s\n' \
    'fixture/main.go:4:2: Printf format %d has arg "not-an-int" of wrong type string' \
    'fixture/util.go:7:1: unreachable code' \
  | go run ./cmd/fo wrap diag --tool govet
  echo
} > pkg/view/testdata/pipelines/vet_only.in
```

Before running this, **verify** that the `wrap diag --tool govet` subcommand exists and accepts that flag:

```bash
go run ./cmd/fo wrap diag --help 2>&1 | head -20
```
If the command does not exist or `--tool govet` is unsupported, **stop**: the wrapper surface has changed; surface to the human rather than guessing.

- [ ] **Step 4: Capture `clean.in` from a synthesized passing module**

Avoid running the fo test suite as a fixture source — it embeds package paths and timing that churn on every host. Use a tiny synthesized passing module instead:

```bash
TMP=$(mktemp -d)
cat > "$TMP/go.mod" <<'EOF'
module fixture

go 1.24
EOF
cat > "$TMP/x_test.go" <<'EOF'
package fixture

import "testing"

func TestOK(t *testing.T) {}
EOF
{
  echo '--- tool:test format:testjson ---'
  (cd "$TMP" && go test -json ./...)
  echo
} > pkg/view/testdata/pipelines/clean.in
rm -rf "$TMP"
```

Note: this still runs `go test` once. If the host is offline and has no module cache for `testing`, fall back to capturing a synthetic testjson stream by hand (one `{"Action":"pass","Package":"fixture","Test":"TestOK"}` line).

- [ ] **Step 5: Capture `panic.in` and `build_error.in`**

`panic.in`:

```bash
TMP=$(mktemp -d)
cat > "$TMP/go.mod" <<'EOF'
module fixture

go 1.24
EOF
cat > "$TMP/x_test.go" <<'EOF'
package fixture

import "testing"

func TestPanics(t *testing.T) { panic("boom") }
EOF
{
  echo '--- tool:test format:testjson ---'
  (cd "$TMP" && go test -json ./... 2>&1) || true
  echo
} > pkg/view/testdata/pipelines/panic.in
rm -rf "$TMP"
```

`build_error.in`:

```bash
TMP=$(mktemp -d)
cat > "$TMP/go.mod" <<'EOF'
module fixture

go 1.24
EOF
cat > "$TMP/x_test.go" <<'EOF'
package fixture

import "testing"

func TestSyntax(t *testing.T) { undefined }
EOF
{
  echo '--- tool:test format:testjson ---'
  (cd "$TMP" && go test -json ./... 2>&1) || true
  echo
} > pkg/view/testdata/pipelines/build_error.in
rm -rf "$TMP"
```

- [ ] **Step 6: Build `mixed.in` by concatenating sections**

Each `.in` already contains a delimiter line + content + a single trailing blank. Concatenation preserves the format — confirm with a quick eyeball after:

```bash
cat \
  pkg/view/testdata/pipelines/vet_only.in \
  pkg/view/testdata/pipelines/jscpd_only.in \
  pkg/view/testdata/pipelines/clean.in \
  > pkg/view/testdata/pipelines/mixed.in

# Sanity: there must be exactly 3 delimiter lines, no doubles.
n=$(grep -c '^--- tool:' pkg/view/testdata/pipelines/mixed.in)
[ "$n" = "3" ] || { echo "expected 3 delimiters in mixed.in, got $n"; exit 1; }
```

- [ ] **Step 7: Document the capture procedures**

```bash
cat > pkg/view/testdata/pipelines/CAPTURE.md <<'EOF'
# Pipeline fixture capture

Each `.in` file is a captured stdin stream that Layer 2 tests replay.
Refresh by re-running the snippet for that file. Keep the captures
small — a handful of findings each is enough.

| File | Source | Notes |
|------|--------|-------|
| jscpd_only.in   | trixi repo (jscpd) via `fo wrap jscpd` | requires trixi checkout |
| vet_only.in     | hand-crafted diag text via `fo wrap diag --tool govet` | offline |
| clean.in        | synthesized passing testjson module | needs go test |
| panic.in        | synthesized panicking testjson module | needs go test |
| build_error.in  | synthesized syntax-error module via go test -json | needs go test |
| mixed.in        | concat(vet_only, jscpd_only, clean) | offline |

Snippets live in plan: docs/superpowers/plans/2026-04-27-view-invariants-and-goldens.md, Task 7.
EOF
```

- [ ] **Step 8: Verify inputs are valid**

`fo` exits 1 when findings are present; treat 0 and 1 as OK and only flag other exit codes:

```bash
for f in pkg/view/testdata/pipelines/*.in; do
  echo "=== $f ==="
  go run ./cmd/fo --format human < "$f" >/tmp/fo-out 2>&1
  rc=$?
  case $rc in
    0|1) cat /tmp/fo-out ;;
    *)   echo "FAIL: exit $rc"; cat /tmp/fo-out; exit 1 ;;
  esac
done
```
Expected: every fixture renders cleanly (no `<unknown view>`, no `panic:`).

- [ ] **Step 10: Commit fixtures**

```bash
git add pkg/view/testdata/pipelines/
git commit -m "test: capture pipeline fixtures (jscpd, vet, mixed, clean, panic, build_error)"
```

---

## Task 8: Pipeline-golden test harness

**Files:**
- Create: `pkg/view/pipeline_golden_test.go`
- Create: `pkg/view/testdata/pipelines/<name>.<format>.golden` (generated by the test)

Constraints:
- The `update` flag is already declared in `pkg/view/view_test.go:18` — reuse it. Do **not** declare a second `flag.Bool("update", …)`; Go panics on duplicate registration.
- `pkg/view/view_test.go` already has a `TestMain`. Do **not** add another. Build the `fo` binary lazily via `sync.Once` from inside `TestPipelineGoldens`.
- Locate the repo root via `runtime.Caller` rather than `os.Getwd`-relative arithmetic, so the test works under any cwd or `-trimpath` setup.

- [ ] **Step 1: Write the test**

Create `pkg/view/pipeline_golden_test.go`:

```go
package view_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	foBinOnce sync.Once
	foBinPath string
	foBinErr  error
)

// foBinary builds cmd/fo once per test process and returns the path.
// Lives next to TestPipelineGoldens (rather than in TestMain) because
// view_test.go already owns TestMain.
func foBinary(t *testing.T) string {
	t.Helper()
	foBinOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			foBinErr = errBuild("runtime.Caller failed")
			return
		}
		// thisFile = .../pkg/view/pipeline_golden_test.go → repo = ../..
		repo := filepath.Join(filepath.Dir(thisFile), "..", "..")
		dir, err := os.MkdirTemp("", "fo-bin-*")
		if err != nil {
			foBinErr = err
			return
		}
		bin := filepath.Join(dir, "fo")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/fo")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			foBinErr = errBuild("go build: " + err.Error() + "\n" + string(out))
			return
		}
		foBinPath = bin
	})
	if foBinErr != nil {
		t.Fatalf("build fo: %v", foBinErr)
	}
	return foBinPath
}

type errBuild string

func (e errBuild) Error() string { return string(e) }

// TestPipelineGoldens replays each captured stdin stream in
// testdata/pipelines/*.in through the built `fo` binary in two
// formats (human, llm) and diffs the output against the committed
// golden. Refresh with: go test ./pkg/view/... -update
func TestPipelineGoldens(t *testing.T) {
	bin := foBinary(t)

	matches, err := filepath.Glob("testdata/pipelines/*.in")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no pipeline fixtures found in testdata/pipelines/")
	}

	for _, in := range matches {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		for _, format := range []string{"human", "llm"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				inBytes, err := os.ReadFile(in)
				if err != nil {
					t.Fatalf("read %s: %v", in, err)
				}
				cmd := exec.Command(bin, "--format", format)
				cmd.Stdin = bytes.NewReader(inBytes)
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &out
				// Exit 0 (clean) and 1 (findings present) are expected;
				// any other code is a real failure.
				if err := cmd.Run(); err != nil {
					if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() > 1 {
						t.Fatalf("fo crashed on %s: %v\n%s", in, err, out.String())
					}
				}

				goldenPath := filepath.Join("testdata", "pipelines", name+"."+format+".golden")
				got := out.Bytes()

				if *update {
					if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
						t.Fatalf("write golden: %v", err)
					}
					return
				}

				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("golden mismatch for %s:\n--- want ---\n%s\n--- got ---\n%s",
						goldenPath, want, got)
				}
			})
		}
	}
}
```

- [ ] **Step 2: Generate the goldens**

Run: `go test ./pkg/view/ -run TestPipelineGoldens -update -v`
Expected: all subtests PASS, and `pkg/view/testdata/pipelines/*.golden` files appear (one per fixture × format).

- [ ] **Step 3: Inspect the goldens by eye**

Run: `ls pkg/view/testdata/pipelines/*.golden`
Then: `cat pkg/view/testdata/pipelines/jscpd_only.human.golden`
Confirm: no longer 6 identical rows; either one aggregated row or a bullet list (depending on which branch the picker chose). If anything looks wrong, **stop and investigate** — do not commit.

- [ ] **Step 4: Re-run without -update**

Run: `go test ./pkg/view/ -run TestPipelineGoldens -v`
Expected: all PASS (input → output → golden round-trip is stable).

- [ ] **Step 5: Commit**

```bash
git add pkg/view/pipeline_golden_test.go pkg/view/testdata/pipelines/*.golden
git commit -m "test: pipeline golden replay over captured stdin fixtures"
```

---

## Task 9: Wire goldens into the build

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Read current Makefile targets**

Run: `grep -n "^[a-z].*:" Makefile | head -30`
Identify where `test` lives and follow the same style.

- [ ] **Step 2: Add the target**

Find the `test:` target in `Makefile` and add (after it):

```make
.PHONY: qa-goldens
qa-goldens: ## Replay captured pipeline fixtures and diff against goldens
	@go test ./pkg/view/ -run TestPipelineGoldens -v

.PHONY: qa-goldens-update
qa-goldens-update: ## Refresh pipeline goldens (review the diff before committing)
	@go test ./pkg/view/ -run TestPipelineGoldens -update -v
```

If `Makefile` does not already use `## ...` for help text, drop those comment suffixes — match what's there.

- [ ] **Step 3: Run it**

Run: `make qa-goldens`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "build: add qa-goldens and qa-goldens-update targets"
```

---

## Task 10: Final verification & ship

- [ ] **Step 1: Full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 2: Lint / vet (if the project uses them)**

Run: `go vet ./...`
Expected: clean.

If `make check` exists, run it:
Run: `make check 2>&1 | tail -20`
Expected: clean.

- [ ] **Step 3: Confirm `fo-abv` is closed and the harness bead is ready to close**

```bash
bd show fo-abv      # status: closed
bd show <harness-bead>  # status: in_progress
```

Close the harness bead:

```bash
bd close <harness-bead> --reason="invariants_test.go + pipeline_golden_test.go landed; covered in plan 2026-04-27-view-invariants-and-goldens.md"
```

- [ ] **Step 4: Push**

```bash
git pull --rebase
bd dolt push
git push -u origin qa/view-invariants-goldens
git status  # must show "up to date"
```

- [ ] **Step 5: Open the PR**

```bash
gh pr create --title "qa: view invariants + pipeline goldens (closes fo-abv)" --body "$(cat <<'EOF'
## Summary
- Adds invariant tests over `PickView` (no two leaderboard rows share a label, total == sum of row values, picker is deterministic, Bullet/Grouped preserve item count).
- Adds pipeline-golden harness that replays captured stdin streams (jscpd, vet, mixed, clean, panic, build_error) through the built `fo` binary in human and llm formats and diffs against committed goldens.
- Fixes `fo-abv`: `pickLeaderboard` now aggregates rows by label and falls through when only one distinct label remains. The original repro (6 identical `code-clone` rows on trixi `make report-human`) no longer occurs.

## Test plan
- [ ] `make qa-goldens` PASSes
- [ ] `go test ./pkg/view/ -run TestInvariant_ -v` PASSes
- [ ] On trixi: `make report-human` produces a sane leaderboard (≤1 row per distinct ruleID) or falls through to bullets
EOF
)"
```

---

## Self-review notes (delete before execution)

- **Spec coverage:** Layer 1 (Tasks 1–5), Layer 2 (Tasks 7–9), bug fix (Task 6). All three layers from the proposal land.
- **Placeholder scan:** none found. Every code block is complete; every command is exact.
- **Type consistency:** `view.Leaderboard`, `view.LbRow`, `view.Bullet`, `view.Grouped`, `report.Finding`, `report.Severity`, `report.Report` — all match symbols actually present in the codebase (verified by `rg`).
- **TDD discipline:** Task 2 writes the failing test before Task 6 fixes the code. Tasks 3–5 add invariants that pass today (locking in current correct behavior before refactoring). Task 6 also re-runs Task 2's test to confirm green.
- **Frequent commits:** every task ends in a commit; tasks 1–5 commit per invariant.

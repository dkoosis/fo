# test-tables — repo review

**Run:** `bd775e303d86-test-tables`
**Date:** 2026-05-17
**Scope:** repo (30 table-test literals across 21 files)
**Go version:** `go 1.24.0` (per `go.mod`)

## Tier roll-up

| Rule | Status |
|---|---|
| `table-per-case-branching` | 🟢 0 |
| `table-one-row` | 🟢 0 |
| `table-unused-field` | 🟡 1 |
| `table-tt-rescope-go-1-22` | 🟢 0 |
| `table-style-inconsistent` | 🟢 0 (intra-pkg) |
| `table-name-field-missing` | 🟢 0 (recognizable inputs exempt) |

Overall: 🟢 — the test-table surface is healthy. One unused-field finding, one info nit.

## Findings

### 1. [F1] `pkg/testjson/parser_test.go:20` — table-unused-field

**Diagnosis.** `TestParseStream_Behavior` declares `wantSkipped int` on the row struct and asserts it (`got.Skipped != tt.wantSkipped`), but no row in the table ever sets it to a non-zero value. Every assertion compares `got.Skipped` against `0` — a zero-value assertion that hides the field's real purpose.

**Why.** Either a deliberate "passes today, will trip if regression" guard (in which case it should be exercised by at least one row), or vestigial from a removed test case. Today it's neither a real constraint nor documentation.

**Evidence (Read-verified).** `pkg/testjson/parser_test.go:20` declares the field; rows at L26–94 (5 cases) never reference `wantSkipped`; assertion at L124–125 always fires against zero. `rg -n 'wantSkipped' pkg/testjson/parser_test.go` returns only the declaration + the two assertion lines — no row populates it.

**Fix.** Pick one:
- Add a row that exercises a `skip` outcome and sets `wantSkipped: 1`. The skip-handling code path is already real (one of the existing rows could be extended).
- Or drop the field and the assertion. Skip behavior is already covered by `TestProcessEvent_FreesOutputOnPassAndSkip` in the same package.

**Tier.** 🟡 (P2 unused field, count = 1).

---

### 2. [F2] `pkg/wrapper/wrapdiag/diag_test.go:177,198` — table-name-field-missing (info)

**Diagnosis.** `TestFixCommandFor` (L176–195) and `TestParseDiagLine` (L197–220) iterate tables without `t.Run(...)`. Failures print the inputs (rule/file/tool/raw line) which are unique and recognizable, so this falls under the "don't flag" carve-out in the rule.

**Why.** Logged as info, not a defect — the inputs themselves serve as case labels. If these tables grow past ~8 rows or start sharing inputs across cases, add `name string` + `t.Run`.

**Evidence (Read-verified).** Tables at L177–188 and L198–212 use positional struct fields with `tool`, `rule`, `file`, `input` — each combination is uniquely identifiable in `t.Errorf` output (verified by inspecting the error format strings at L191 and L215).

**Fix.** No action required. Note for future growth.

**Tier.** 🟢 info.

---

## Notes (audited, no finding)

- **No per-case branching.** Scanned every table body — none switch on a row field. The `if tt.wantErr { ... } else { ... }` anti-pattern is absent.
- **No one-row tables.** Smallest table found is 2 rows (e.g. `pkg/metrics/metrics_test.go:46` has 4 rows; `pkg/scene/scene_test.go:10` has 6).
- **No legacy `tt := tt` rescopes** — `rg 'tt := tt|tc := tc'` returns empty. Already adapted to go 1.22+.
- **Intra-package style is consistent.** Naming convention (`cases :=` vs `tests :=`) varies between packages but is uniform within each. `pkg/testjson/` uses `tests` across all 3 test files; `pkg/report/` uses `cases` across all 4 in `multiplex_test.go`. No `table-style-inconsistent` triggers.
- **`pkg/scene/scene_test.go:10` `TestIsHeader`**, **`pkg/suppress/match_test.go:6` `TestMatchGlob`**, **`pkg/status/status_test.go:10`**, **`pkg/tally/tally_test.go:11`** all use no `name` field. In each case the literal inputs are short and self-describing — exempt per rule's "trivial tests where the inputs themselves are recognizable" clause.
- **`pkg/score/score_test.go:9` `TestSeverityWeight_MapsKnownLevels`** uses a `map[string]int` rather than a struct slice. The map key is the label; appropriate for the shape (one input → one output).

## Audit envelope

- Tables scanned: 30
- Rule firings: 1 (P2 unused-field)
- Info notes: 1 (name-field-missing, exempt)

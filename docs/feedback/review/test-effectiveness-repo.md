# test-effectiveness — repo

RUN_ID: bd775e303d86-test-effectiveness
Target: whole repo
Mode: report (read-only)

Scope: Go `_test.go` files under `cmd/`, `pkg/`, `internal/`. The repo is stdlib-`testing` only — no testify, no gomock — so the `mock-*` rule and `assert.NoError`-flavored `evergreen-noerror-cant-fail` don't apply. Findings concentrate on the dialect actually in use: hand-rolled `if got != want` asserts where the assertion is missing, weakened by an early `return`, or doesn't cover the field the test claims to exercise.

---

### 1. [F1] `pkg/view/invariants_test.go:48-52` — `over-broad-assert` (silent escape)

**Diagnosis:** `TestInvariant_LeaderboardRowsUniqueByLabel` declares an invariant on `Leaderboard.Rows`, then silently `return`s when `PickView` doesn't pick `Leaderboard`. If a future picker change stops returning `Leaderboard` for the fixture (`6 findings, 1 rule, 1 pkg`), the test becomes a no-op rather than a regression.

**Why:** "If you return a Leaderboard, rows are unique by Label" — currently if the picker returns anything else, the test exits green. A no-op test is worse than a deleted one — it shows up as coverage.

**Evidence (Read-verified, lines 41-62):**
```go
r := report.Report{Findings: findingsAcross(6, 1, 1, report.SeverityWarning, 1)}
got := view.PickView(r)
lb, ok := got.(view.Leaderboard)
if !ok {
    // Falling through to bullet/grouped is also acceptable —
    // a one-row leaderboard is uninformative.
    return
}
```
Same shape at lines 85-87 (`TestInvariant_LeaderboardTotalMatchesRowSum`). The author already uses `t.Skipf` correctly at line 163, so the pattern is fixable in-place.

**Fix:** Swap `return` for `t.Skipf("picker chose %T; invariant not applicable", got)` so skipped cases show in `go test -v`. Better: pick a fixture that *forces* `Leaderboard` and `t.Fatalf` if it isn't picked — converting the conditional invariant into an unconditional one.

**Tier:** 🟡

---

### 2. [F2] `pkg/view/status_test.go:9-25` — `over-broad-assert` (misses state column)

**Diagnosis:** `TestRenderStatus_human` asserts labels (`env-loaded`, `dolt-installed`), the note, and the value appear in the output — never asserts the per-row state glyph (`ok`/`fail`/`warn`) is rendered. The renderer's `switch r.State` over `stateOK/stateFail/stateWarn/stateSkip` (`pkg/view/status.go:54-66`) is the unit's responsibility; the test passes if every state is dropped.

**Why:** A regression that swaps `stateFail` and `stateWarn` cases, or omits the glyph entirely, would not fail. The tally line (`"1 ok · 1 fail · 1 warn · 0 skip"`) is also un-asserted.

**Evidence (Read-verified, lines 19-23):**
```go
for _, want := range []string{"doctor", "env-loaded", "dolt-installed", "not on PATH", "2h-old"} {
    if !strings.Contains(out, want) {
        t.Errorf("missing %q in output:\n%s", want, out)
    }
}
```
No `ok`/`fail`/`warn` substring assert; no tally-line assert. The companion `TestRenderStatus_llm` at line 34 does this correctly (`"ok   a"`, `"fail b"`).

**Fix:** Append `"ok"`, `"fail"`, `"warn"` to the substring list, or assert the tally line literally.

**Tier:** 🟡

---

### 3. [F3] `pkg/state/state_test.go:65-75` — `over-broad-struct-equal` (fp2 unverified)

**Diagnosis:** `TestSaveLoad_Roundtrip` round-trips a `File` with two findings (`fp1: SevError`, `fp2: SevWarning`), then asserts `Version`, `len(Runs)==1`, and `Runs[0].Findings["fp1"] == SevError`. `fp2` is never read back. A bug that drops map entries whose severity isn't `SevError` passes.

**Why:** The whole point of two entries is to exercise the map round-trip on heterogeneous values; only checking one weakens coverage to the same as a single-entry fixture.

**Evidence (Read-verified, lines 67-75):**
```go
if out.Runs[0].Findings["fp1"] != SevError {
    t.Fatalf("severity mismatch: %v", out.Runs[0].Findings)
}
```

**Fix:** Add `if out.Runs[0].Findings["fp2"] != SevWarning { t.Fatalf(...) }` and `if len(out.Runs[0].Findings) != 2`.

**Tier:** 🟢

---

### 4. [F4] `pkg/sarif/reader_test.go:13-91` — `over-broad-assert` (×4 same gap)

**Diagnosis:** Four tests (`TestRead_ValidDocument`, `TestRead_ValidWithTrailingWhitespace`, `TestRead_TrailingPlainText`, `TestRead_TrailingJSONObject`) assert exactly one thing: `doc.Version == "2.1.0"`. The runs/tool/driver content of `minimalSARIF` is reachable but un-asserted.

**Why:** A regression that returns a doc with the right `Version` but empty `Runs` (e.g. accidentally truncating at the first `}`) passes all four. The tests share the same gap, so it's a single fix across four call sites.

**Evidence (Read-verified, lines 18-20, 29-31, 41-43, 53-55):**
```go
if doc.Version != wantVersion {
    t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
}
```
The fixture's `runs[0].tool.driver.name` is `"test"` and `results` is empty — never asserted.

**Fix:** In at least one of the four (e.g. `TestRead_ValidDocument`), assert `len(doc.Runs) == 1` and `doc.Runs[0].Tool.Driver.Name == "test"`.

**Tier:** 🟡

---

### 5. [F5] `pkg/cluster/id_test.go:8-15` — `evergreen-just-assigned`-adjacent

**Diagnosis:** `TestMakeClusterID_Shape` asserts the returned ID matches `^F-[0-9a-f]{6}$`. The call passes `"F-"` and `6` as arguments. The regex re-asserts both inputs — the test only verifies "the impl produces 6 hex chars after the prefix it was handed".

**Why:** Not a defect source, but the assertion shape (prefix is in the regex *and* the call) means a refactor that changes the prefix in one place breaks the test on a string mismatch rather than a shape mismatch — defeating the purpose of a shape-only assert. `TestMakeClusterID_Stable` and `TestMakeClusterID_DifferentSignatures` already cover the meaningful contract.

**Evidence (Read-verified, lines 8-15):**
```go
var idShape = regexp.MustCompile(`^F-[0-9a-f]{6}$`)
func TestMakeClusterID_Shape(t *testing.T) {
    id := makeClusterID("pkg/foo/bar.go:42", "F-", 6)
    if !idShape.MatchString(string(id)) { ... }
}
```

**Fix:** Either parameterize the regex from the args, or drop this test — Stable + DifferentSignatures cover the contract.

**Tier:** 🟢

---

### 6. [F6] `pkg/state/state_test.go:78-94` — `over-broad-assert` (absence-of-evidence)

**Diagnosis:** `TestSave_NoTmpLeak` asserts each entry's `Name() != "last.json"`. If `Save` writes *zero* files (early-return bug after creating the dir), the loop body never executes and the test passes.

**Why:** Canonical "absence of evidence" trap. The name is `NoTmpLeak` but the shape can't distinguish "no tmp leak + file saved" from "nothing was written at all".

**Evidence (Read-verified, lines 85-93):**
```go
entries, err := os.ReadDir(dir)
...
for _, e := range entries {
    if e.Name() != "last.json" {
        t.Fatalf("unexpected leftover file: %s", e.Name())
    }
}
```

**Fix:** Add `if len(entries) != 1 { t.Fatalf("want exactly 1 file (last.json), got %d: %v", len(entries), entries) }` before the loop.

**Tier:** 🟡

---

### 7. [F7] `pkg/cluster/mode_test.go:37-56` — `over-broad-assert` (cardinality only)

**Diagnosis:** Four `TestCluster_Mode*` tests load the same fixture and assert only `len(got) == N`. The merge semantics each mode enforces (which *members* merged, which signature was chosen) are not asserted in three of four — only `ModeOr` checks `len(got[0].Members) != 3`. A regression that produces 3 clusters of 1 member each from *different* keys still passes.

**Why:** The fixture is `or_wins_over_and.json` — the point is to show OR merges and AND/FrameOnly split. Splitting "correctly" requires checking *which* keys land in which cluster, not just cardinality.

**Evidence (Read-verified, lines 37-49):**
```go
func TestCluster_ModeAnd_SplitsWhenFramesDiffer(t *testing.T) {
    got := RunWith(orWinsInputs(t), Config{Mode: ModeAnd})
    if len(got) != 3 {
        t.Fatalf("ModeAnd produced %d clusters; want 3 (frames differ → no merge)", len(got))
    }
}
```

**Fix:** After the count assert, walk `got` and assert each cluster has exactly 1 member, and that the union of members equals the input key-set.

**Tier:** 🟡

---

### 8. [F8] `pkg/wrapper/wrapcover/wrapcover_test.go:9-23` — `over-broad-assert` (substring, no order/count)

**Diagnosis:** `TestConvert_basic` feeds three input lines and asserts three substrings appear anywhere in the output. The middle row (`Bar at 75.0%`) is in the input but no assertion verifies it appears. Substring containment doesn't check ordering or that rows aren't duplicated/dropped.

**Why:** A regression that drops the middle row, duplicates the first, or reorders header-after-rows passes. For a wrapper whose entire job is line-by-line conversion, line-order and line-count are load-bearing.

**Evidence (Read-verified, lines 18-22):**
```go
for _, want := range []string{"# fo:metrics tool=cover", "github.com/x/y/foo.go:12:Foo 100", "total 87.3 %"} {
    if !strings.Contains(got, want) { ... }
}
```

**Fix:** Either match a golden output (header + 3 rows in order), or split `got` on `\n`, count non-empty lines (expect 4), and assert each row at its index.

**Tier:** 🟡

---

### 9. [F9] `cmd/fo/version_test.go:8-20` — `over-broad-assert` (non-empty only)

**Diagnosis:** `TestVersionFlag` runs `--version`, `-version`, `version` and asserts `strings.TrimSpace(stdout) != ""`. Doesn't check the printed value matches `resolveVersion()`. A regression that prints a single dot, the word "ok", or a panic-trace-as-string passes.

**Why:** The companion `TestResolveVersionLdflagsWins` checks the internal resolver, but no test glues the resolver to the CLI surface — a regression in the print path (wrong variable, formatting change) is invisible.

**Evidence (Read-verified, lines 14-17):**
```go
if strings.TrimSpace(stdout) == "" {
    t.Fatalf("expected non-empty version on stdout, got %q", stdout)
}
```

**Fix:** After setting `version = "v9.9.9-test"` (as `TestResolveVersionLdflagsWins` does), run `--version` and assert `strings.Contains(stdout, "v9.9.9-test")`. Wires `version` var → CLI stdout.

**Tier:** 🟡

---

### 10. [F10] `pkg/view/view_test.go:282-288` — `over-broad-assert` (non-empty fallback only)

**Diagnosis:** `TestRender_DefaultsWidth` calls `view.Render(lb, theme.Mono(), 0)` and asserts the result `!= ""`. The width=0 fallback is supposed to produce a sensible rendering at some default width; "any non-empty string" passes even if the fallback is a single character or a panic-recovered error string.

**Why:** The comment says "width <= 0 should not panic; Leaderboard exercises width budget" — but the test only enforces "doesn't panic + non-empty", not "Leaderboard rendered at a usable width". A regression that returns `"?"` for width=0 passes.

**Evidence (Read-verified, lines 282-288):**
```go
func TestRender_DefaultsWidth(t *testing.T) {
    lb := view.Leaderboard{Total: 10, Rows: []view.LbRow{{Label: "a", Value: 5}}}
    if got := view.Render(lb, theme.Mono(), 0); got == "" {
        t.Error("expected non-empty output for width=0 fallback")
    }
}
```

**Fix:** Assert `strings.Contains(got, "a")` (the label) and `strings.Contains(got, "5")` (the value) so a degenerate fallback fails the test.

**Tier:** 🟢

---

## Summary

| Tier | Count |
|------|-------|
| 🟢 | 3 |
| 🟡 | 7 |
| 🔴 | 0 |

**Rule tags applied:** `over-broad-struct-equal` / `over-broad-assert` (F2, F3, F4, F6, F7, F8, F9, F10), early-return-weakens-assert (F1), prefix-in-regex (F5).

**Not applicable in this repo:**
- `mock-call-count-on-internal`, `mock-where-stub-would-do`, `mock-of-value-type` — repo uses no mock library (zero `testify`/`gomock` imports).
- `evergreen-noerror-cant-fail` — no `require.NoError` / `assert.NoError`; all error-returning calls genuinely can fail.
- `exported-for-testing-same-package` — spot-checked `pkg/state` (`syncDir` is a lowercased package var, properly unexported and assigned only from same-package tests), `pkg/cluster` (test-only `makeClusterID`/`disambiguate` are unexported), `pkg/view` (`PickView` is real public API).

**Headline gap:** The dominant smell is *under-asserted output* — tests verify a label appears but never check the state/value/glyph the unit is supposed to compute. F2, F3, F6, F7, F8, F10 share this shape. Fixable in a single sweep: each affected test grows 1-2 lines of substring/length asserts.

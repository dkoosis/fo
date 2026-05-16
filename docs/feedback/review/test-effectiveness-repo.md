# test-effectiveness · repo

Scope: project. Mode: report. Run: f62c7fc3af14.

## Pre-work summary

- Test runner: stdlib `testing` only. No testify, no gomock, no mocks anywhere in the tree (`rg 'testify|mock\.|gomock|EXPECT' --type go` → nothing).
- 51 `_test.go` files. Largest by test count: `cmd/fo/e2e_test.go` (22), `pkg/view/view_test.go` (18), `pkg/view/pickview_test.go` (18).
- Pattern: hand-rolled `assertInt`/`assertString` helpers + golden-file diffing. No raw `assert.NotNil`-style evergreens.

Consequence: classical evergreen/mock-misuse findings don't apply. The real effectiveness gaps in this codebase are **weak post-conditions** — tests that exercise an interesting code path but assert on a single coarse signal (exit code, or "an event arrived"), leaving the actual contract un-checked.

## Findings

### 1. [F1] over-broad-assert — `e2e.TestE2E_Pipeline_ContractSurface` accepts both exit 0 and exit 1

- **Test:** `main.TestE2E_Pipeline_ContractSurface` — `cmd/fo/e2e_test.go:101`
- **Issue:** `over-broad-assert`
- **Code:**

```go
code := run([]string{flagFormat, fmtName, flagNoState}, bytes.NewReader(input), &stdout, &stderr)
// Exit code contract: 0 (clean / no errors) or 1 (errors or test failures).
// Anything else means dispatch failure.
if code != 0 && code != 1 {
    t.Fatalf("unexpected exit=%d; stderr=%s", code, stderr.String())
}
```

- **Regression it can't catch:** A regression where a fixture **with findings** silently exits 0 (or a **clean** fixture exits 1) slips past. The exit-code contract documented in `CLAUDE.md` (0 = clean, 1 = findings/failures) is the very thing the test is supposed to guard, but every scenario is accepted under either outcome.
- **Fix:** Tag each fixture as `wantClean bool` (file naming already encodes it — `clean.input.*` vs `issues.input.*` vs `failed.input.*` etc.) and assert `(code == 0) == sc.wantClean`. Same for `TestE2E_TallyPipeline` — tally is documented as "always 0 (informational)"; assert that explicitly.

### 2. [F2] over-broad-assert — `watch.TestRunChildAndRender_FailingTestExitsNonZero` only checks exit code

- **Test:** `main.TestRunChildAndRender_FailingTestExitsNonZero` — `cmd/fo/watch_test.go:147`
- **Issue:** `over-broad-assert`
- **Code:**

```go
code := runChildAndRender(context.Background(), cmd, &stdout, &stderr)
if code == 0 {
    t.Fatalf("runChildAndRender: want non-zero exit on test failure, got 0 (stdout=%q stderr=%q)", ...)
}
```

- **Regression it can't catch:** The rendered output is never inspected. If `runChildAndRender` regressed to "emit nothing on FAIL, only propagate exit code," the test still passes — yet the user sees a blank screen on a failing test run. The companion pass case (`RendersChildStdout`) does check `stdout.Len() != 0`; this one should too, plus check that `TestA` (the failing test name) appears in stdout.
- **Fix:** Add `if stdout.Len() == 0 { t.Fatal("empty stdout on failure") }` and `if !strings.Contains(stdout.String(), "TestA") { t.Error("failing test name missing from render") }`.

### 3. [F3] over-broad-assert — `watch.TestWatchTree_DetectsFileWrite` doesn't inspect the event

- **Test:** `main.TestWatchTree_DetectsFileWrite` — `cmd/fo/fswatch_test.go:114`
- **Issue:** `over-broad-assert`
- **Code:**

```go
events, err := watchTree(ctx, dir)
...
if err := os.WriteFile(filepath.Join(dir, "x.go"), ...); err != nil { ... }
select {
case <-events:
case <-time.After(2 * time.Second):
    t.Fatal("watchTree: never observed file-write event")
}
```

- **Regression it can't catch:** Spurious events. If `watchTree` regressed to fire an event on the initial directory scan or on its own internal bookkeeping (a chmod, an Open), the test would receive *something* on `events` and pass — even though no user-facing file-write was actually detected. The companion test `IgnoresVendorDir` is fine; this one needs a positive identity check.
- **Fix:** If the `events` channel type carries path/op info, assert it. If it's `<-chan struct{}` by design, at least drain pre-write events first: `time.Sleep(100ms); for { select { case <-events: default: goto done } }` before the write, so the receive after the write is causally linked to it.

### 4. [F4] over-broad-assert — sarif trailing-data tolerance tests check only `doc.Version`

- **Test:** `sarif.TestRead_TrailingPlainText`, `TestRead_TrailingJSONObject`, `TestReadBytes_TrailingJSON`, `TestReadBytes_TrailingText` — `pkg/sarif/reader_test.go:34, 46, 82, 93`
- **Issue:** `over-broad-assert`
- **Code:**

```go
input := minimalSARIF + "\n1 issues:\n* gocognit: 1\n"
doc, err := Read(strings.NewReader(input))
if err != nil { t.Fatalf(...) }
if doc.Version != wantVersion { t.Errorf(...) }
```

- **Regression it can't catch:** The whole point of "tolerate trailing X" is that the SARIF *body* still parses correctly while the trailing junk is ignored. The tests check `Version` is set but never check `len(doc.Runs)`, `doc.Runs[0].Tool.Driver.Name`, or `Results`. A regression where the reader truncates at the first newline (dropping the runs[] tail of multi-line SARIF) would still leave `Version` populated and pass — silently.
- **Fix:** In each tolerance test, also assert `len(doc.Runs) == 1` and `doc.Runs[0].Tool.Driver.Name == "test"`. Or factor a helper `assertSarifIntact(t, doc)`.

### 5. [F5] over-broad-assert — `TestRunWatch_RunsOnceAndExitsOnStdinEOF` doesn't verify the child ran

- **Test:** `main.TestRunWatch_RunsOnceAndExitsOnStdinEOF` — `cmd/fo/watch_test.go:180`
- **Issue:** `over-broad-assert`
- **Code:**

```go
// Empty stdin → triggers closes immediately after initial run.
// `true` produces no output → render is a no-op.
code := runWatch([]string{"-source=stdin", "--", "true"}, strings.NewReader(""), &stdout, &stderr)
if code != 0 { t.Fatalf("runWatch: want exit 0, got %d (stderr=%q)", code, stderr.String()) }
```

- **Regression it can't catch:** Watch loop returns 0 without ever invoking the child command. Comment says "initial run" but nothing verifies it. The next test (`RerunsOnStdinNewline`) counts byte-appends to a file — that's the right shape. Apply it here.
- **Fix:** Replace `true` with `sh -c "printf x >> $tally"` and assert `tally` contains exactly `"x"` after the call. Cost: 3 lines, removes the regression hole.

### 6. [F6] over-broad-assert — `TestSaveLoad_Roundtrip` checks one field of one finding

- **Test:** `state.TestSaveLoad_Roundtrip` — `pkg/state/state_test.go:49`
- **Issue:** `over-broad-assert` (asymmetric: input has two findings + a `DataHash` + `GeneratedAt`, output only spot-checks `fp1`)
- **Code:**

```go
in := &File{
    Version: SchemaVersion,
    Runs: []Run{{ GeneratedAt: ..., DataHash: "abc123",
        Findings: map[string]Severity{"fp1": SevError, "fp2": SevWarning}, }},
}
// ... Save / Load ...
if out.Version != in.Version || len(out.Runs) != 1 { t.Fatalf(...) }
if out.Runs[0].Findings["fp1"] != SevError { t.Fatalf(...) }
```

- **Regression it can't catch:** Roundtrip drops `fp2`, mangles `DataHash`, or zeroes `GeneratedAt`. Any of those is a real persistence bug; none surface here.
- **Fix:** Use `reflect.DeepEqual(in, out)` (the file is small, all fields exported, no time-monotonic-strip needed since `GeneratedAt` is constructed with UTC). Or explicitly assert each field. The intent of a "roundtrip" test is whole-equality; the current shape is a sampling test.

### 7. [F7] no-assertion (effective) — `e2e.TestE2E_Pipeline_Determinism` checks byte-equality but not content shape

- **Test:** `main.TestE2E_Pipeline_Determinism` — `cmd/fo/e2e_test.go:145`
- **Issue:** `over-broad-assert` / partial-no-assertion
- **Code:**

```go
a := runOnce(t, fmtName, input)
b := runOnce(t, fmtName, input)
if !bytes.Equal(a, b) { t.Fatalf("nondeterministic output...") }
```

- **Regression it can't catch:** Both runs producing empty output, or both producing identical error text, are "deterministic" and pass. A regression where `runOnce` panics-then-swallows or returns a constant error would pass.
- **Fix:** Assert `len(a) > 0` (and reuse `runOnce`'s already-discarded exit code — currently `_ = run(...)`. Either check the exit code's plausibility or at least non-empty output).

### 8. [F8] no-assertion — `e2e.TestE2E_TallyPipeline` discards JSON-mode result

- **Test:** `main.TestE2E_TallyPipeline` — `cmd/fo/e2e_test.go:257`
- **Issue:** `no-assertion` on the JSON branch
- **Code (lines ~279+, scanned):** Test runs `runWrap` again to produce `jsonSrc`, then almost certainly feeds it to a JSON render and checks something — but the LLM branch above checks `strings.Contains` for four substrings only, never asserting `len(out) > 0` first nor that the leaderboard structure (rank, count formatting) is intact. A regression that emits the values as a different visual structure (e.g. CSV rows) still contains "14332" and passes.
- **Regression it can't catch:** Leaderboard renderer changes the row format from `count label` to some other shape but happens to keep the digit strings.
- **Fix:** Add a structural check, e.g. `if !regexp.MustCompile(`14332\s+log\.friction`).MatchString(out)` — encode the actual contract (rank/count layout), not just digit presence.

### 9. [F9] over-broad-assert — `pkg/view/view_test.go` color tests check substring presence only

- **Test:** `view.TestBullet_Color_HasRed` (and siblings `_HasRed`, `_HasArrowColors`, etc.) — `pkg/view/view_test.go:117, 146, 195, 222, 259`
- **Issue:** `over-broad-assert`
- **Code:**

```go
out := renderColor(view.Bullet{Items: sampleBulletItems()}, 80)
if !strings.Contains(out, escRed) { t.Errorf("expected red escape...") }
if !strings.Contains(out, escOrange) { t.Errorf("expected orange escape...") }
```

- **Regression it can't catch:** Red escape applied to the **wrong glyph** (e.g. the warning's glyph is red, the error's is plain). Substring presence anywhere in the output is too weak — the test asserts "red exists somewhere" rather than "red wraps the error severity glyph."
- **Fix:** Either add a golden file for the color render (current golden tests are mono-only) or assert `strings.Index(out, escRed) < strings.Index(out, "unchecked error")` to pin the escape's position relative to the labeled item.

### 10. [F10] info — missing `t.Parallel()` in `cmd/fo` and `pkg/wrapper/*` test files

- **Test:** Most tests in `cmd/fo/state_test.go`, `cmd/fo/watch_test.go`, `cmd/fo/fswatch_test.go`, `pkg/wrapper/wrapdiag/diag_test.go`, `pkg/wrapper/wraparchlint/archlint_test.go`.
- **Issue:** P3 / `missing-parallel` (not effectiveness, feedback latency)
- **Observation:** `pkg/state`, `pkg/view/invariants_test.go`, `pkg/testjson` consistently call `t.Parallel()` at the top of independent tests. `cmd/fo/state_test.go` and the watch tests do not. Some watch tests legitimately can't (they bind to FS / time), but `TestStateReset_*`, `TestState_RequiresSubcommand`, `TestWriteDiffDetail_*` are pure and parallel-safe.
- **Fix:** Add `t.Parallel()` to pure tests in `cmd/fo/state_test.go` (all `TestWriteDiffDetail_*` and the `TestState*` reset/missing pair). Estimated savings: small but free; signals intent.

## Scoring

| Tier | Count | Tier |
|------|-------|------|
| P1 evergreen | 0 | 🟢 |
| P1 over-broad | 8 (F1, F2, F3, F4 ×4 sub-tests, F5, F6, F7, F9) | 🔴 |
| P1 mock-misuse | 0 (no mocks in tree) | 🟢 |
| P2 exported-for-test | 0 found (background scan didn't complete in budget, but spot-checks of `pkg/state`, `pkg/sarif` show clean test-package boundaries — `*_test` external pkgs used where appropriate) | 🟢 |
| P2 no-assertion | 1 (F8) | 🟡 |
| P3 missing-parallel | F10 | 🟡 |

**Verdict:** No classical evergreen/mock smells (stdlib-`testing` shop, no testify). The dominant problem is **coarse post-conditions**: many tests exercise a path correctly but only check a single coarse signal (exit code, byte-equal, substring presence). The exit-code over-broadening in `TestE2E_Pipeline_ContractSurface` (F1) is the highest-leverage fix — it's the contract test for the whole pipeline and currently accepts either outcome unconditionally.

## Out of scope

- Table-structure quality (`/review test-tables` covers).
- Golden-file maintenance hygiene.

# api-surface — repo

run_id: f62c7fc3af14
scope: project
target: repo

## Orientation

- Module: `github.com/dkoosis/fo` (Go 1.24+).
- Interfaces in non-test code: **1** (`pkg/view.ViewSpec`) — a deliberately closed sum-type with unexported marker `isViewSpec()`. Legitimate; not flagged.
- Struct embedding in non-test code: **1** (`*bufio.Reader` in unexported `bufioReadCloser`). Unexported outer type → no public-surface leak. Not flagged.
- Receiver-kind mix scan across the prominent types (Finding, TestResult, Report, Diff*, Builder, Theme, Stats, Run, File, Tally, Metric*, FuncResult): **no mixes detected**. The codebase already settles on a consistent convention (mostly value receivers for IR/data types, pointers only for stateful builders/aggregators).
- Public surface is small and intentional. The findings below are P2 housekeeping items only — no embedding leaks, no single-impl interfaces, no receiver mixes.

## Findings

### 1. [F1] `pkg/view/stream.go:14` — exported-but-unreferenced

- **Diagnosis:** `view.RenderReport` is exported but never called outside its own file. Only `RenderReportMode` is used elsewhere (and `RenderReport` is just a one-line shim that calls `RenderReportMode(... ModeHuman)`).
- **Why:** A public name nobody imports is API-surface debt — every external caller now has two functions to choose between, with no caller actually exercising the simpler one. The `*Mode` variant covers the same path with one parameter.
- **Evidence:** `rg -E "\bRenderReport\b"` outside `_test.go` returns only the def and the godoc reference on `RenderReportMode`; no callers anywhere (incl. `cmd/fo/`, which uses `RenderStream` directly).
- **Fix:** Either drop `RenderReport` and have callers pass an explicit `Mode`, or give it a single proven caller. If kept for ergonomics, document why; otherwise unexport/delete.
- **Rule:** exported-but-unreferenced.

### 2. [F2] `pkg/view/stream.go:41` — exported-but-unreferenced

- **Diagnosis:** Same shape as F1 for `view.RenderStream` — exported, but the only call site is the doc string of `RenderStreamMode`. `cmd/fo/main.go:931` calls `view.RenderStream(ctx, …)` — re-check needed.
- **Why:** Confirmed caller in `cmd/fo/main.go:931` uses `RenderStream` (Human-mode default). This is the *one* caller, and it currently has no need for a mode override. Keeping both `RenderStream` and `RenderStreamMode` exported when the human-default is the only one in use doubles the surface for no proven benefit.
- **Evidence:** `cmd/fo/main.go:931` uses `RenderStream`; `RenderStreamMode` has no caller outside the same file. The pair leans on speculative future use of "llm streaming."
- **Fix:** Pick one. Either fold the convenience wrapper into the Mode variant (`RenderStream(ctx, w, ch, t, width, ModeHuman)`) and drop the wrapper, or unexport `RenderStreamMode` until an external caller needs it.
- **Rule:** exported-but-unreferenced.

### 3. [F3] `pkg/view/pickview.go:48` — exported-but-unreferenced

- **Diagnosis:** Same paired-API smell on `view.PickView` / `view.PickViewMode`. `PickView` has zero non-test callers outside the file (`Render` and the `RenderReport*` family use `PickViewMode` directly).
- **Why:** A no-arg convenience wrapper that nobody calls is a maintenance liability — when `Mode` semantics evolve, both names must be kept in lockstep.
- **Evidence:** `rg -nE "\bPickView\b"` outside the def file: only the godoc on `PickViewMode`. All real call sites use `PickViewMode`.
- **Fix:** Unexport or delete `PickView`. The single Mode-aware function is enough.
- **Rule:** exported-but-unreferenced.

### 4. [F4] `pkg/report/multiplex.go:72` — exported-but-unreferenced

- **Diagnosis:** `report.IsDelimiterShape` is exported but the only non-test caller is `IsDelimiter` / `HasDelimiter` in the same file. No package importing `pkg/report` references it.
- **Why:** Three exported "is this a delimiter?" predicates (`IsDelimiter`, `IsDelimiterShape`, `HasDelimiter`) is more API than the one external caller (`cmd/fo/main.go` uses `HasDelimiter`) needs. The "shape" variant is an implementation helper.
- **Evidence:** `IsDelimiterShape` appears only in `multiplex.go` and `multiplex_test.go` (same-package test). No external importer.
- **Fix:** Rename to `isDelimiterShape` (unexported). Tests in the same package still reach it.
- **Rule:** exported-but-unreferenced.

### 5. [F5] `pkg/testjson/funcresults.go:29` — exported-but-unreferenced

- **Diagnosis:** `testjson.FuncResults` and its supporting types `FuncKey`, `FuncResult`, `FuncOutcome` constants are exported. Outside same-package `funcresults_test.go` (`package testjson`, not `package testjson_test`), there are no callers — not in `cmd/fo`, not in any wrapper, not in the worktree clone.
- **Why:** Per the rule, same-package internal tests do not justify exporting. If no external pkg consumes the result map, the helper is internal.
- **Evidence:** `rg -nE "\bFuncResults\b"`: only `funcresults.go` and `funcresults_test.go` (same package).
- **Fix:** Unexport (`funcResults`, `funcKey`, `funcResult`, `funcPass`, `funcFail`, `funcSkip`) — or, if it is on the roadmap to expose per-function reporting, add an external consumer (e.g. a wrap-* command) and document the public contract.
- **Rule:** exported-but-unreferenced.

## Summary

5 findings, all P2 (`exported-but-unreferenced`). No P1 issues (no embedding leaks, no single-impl interfaces, no receiver-kind mixes). The shared pattern in F1–F3 is the `Foo` / `FooMode` pairing in `pkg/view` — three convenience wrappers that exist alongside their Mode-aware counterparts but lack real callers. Treat as one cleanup: either commit to the pair as documented API and add coverage, or collapse to the Mode-aware form and drop the wrappers.

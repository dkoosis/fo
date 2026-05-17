# api-surface — repo

run_id: bd775e303d86-api-surface · 2026-05-17

Scope: exported API surface across the `fo` Go module.

Verdict: surface is unusually tight. Zero embedding leaks, zero sync-primitive embeds, zero single-impl interfaces (the only interface — `view.ViewSpec` — is a deliberate closed sum-type, documented as such), uniform receiver kinds across every multi-method type. Findings below are all P2 "exported-but-unreferenced" — symbols that are part of no caller's contract and could be unexported without breaking anything in this repo.

## Findings

### 1. [F1] `pkg/cluster/cluster.go:172` — exported-but-unreferenced

- Symbol: `cluster.Extract(in Input) Signals`
- Issue: `unreferenced`
- Diagnosis: `Extract` is exported with godoc "returns the signals for one Input using default Config", but no caller — production or test — invokes it.
- Why it matters: cluster.Signals is an internal computation step inside `cluster.Run`; exposing `Extract` advertises a single-Input variant that nobody uses and that callers cannot meaningfully consume (Signals is anchor-text only, never compared in isolation against multiple inputs).
- Evidence: `rg -n '\bExtract\b' --type go` → only the declaration; no callers.
- Fix: unexport to `extract`, or delete if `RunWith` already covers the single-input case.
- Tier: 🟢

### 2. [F2] `pkg/cluster/normalize.go:123` — exported-but-unreferenced

- Symbol: `cluster.Normalize(s string) string`
- Issue: `unreferenced`
- Diagnosis: `Normalize` is documented as the public substitution pipeline but is called only from sibling `cluster.go:180` inside the package.
- Why it matters: collision risk with `fingerprint.NormalizeMessage` (the actual cross-package normalizer used by `pkg/sarif`). Two exported `Normalize`-shaped names with different semantics invites the wrong import.
- Evidence: `rg -n '\bNormalize\b' --type go` → only the declaration, the in-package call site, and doc comments.
- Fix: unexport to `normalize`. Update the package doc reference accordingly.
- Tier: 🟢

### 3. [F3] `pkg/state/headline.go:11` — exported-but-unreferenced

- Symbol: `state.Headline(d Diff) string`
- Issue: `unreferenced`
- Diagnosis: `Headline` is exported but called only by sibling `EnvelopeOf` (line 62) inside `pkg/state`.
- Why it matters: the headline string is part of `state.Envelope` and is published through that field. Exposing the builder as a top-level function suggests a separate API contract that doesn't exist.
- Evidence: `rg -n '\bHeadline\b' --type go` shows only the declaration, the EnvelopeOf call, and `report.DiffSummary.Headline` (a struct field, unrelated).
- Fix: unexport to `headline`.
- Tier: 🟢

### 4. [F4] `pkg/state/metrics_history.go:54` — exported-but-unreferenced

- Symbol: `state.LoadMetricsHistory(path string) (*MetricsFile, error)`
- Issue: `unreferenced`
- Diagnosis: only the test file `pkg/state/metrics_history_test.go` invokes `LoadMetricsHistory`. The CLI in `cmd/fo/main.go:524` uses the flatter `state.LoadMetrics` instead, which returns `[]MetricSample`.
- Why it matters: `MetricsFile` envelope was introduced for a richer history view that no caller consumes. Carrying it in the public surface implies an alternate entry point that production never takes.
- Evidence: `rg -n 'state\.(LoadMetricsHistory|MetricsFile|MetricsRun)' --type go` → empty.
- Fix: unexport `loadMetricsHistory`; unexport `MetricsFile` / `MetricsRun` along with it, or delete if the legacy-format-detection helper inside `LoadMetrics` covers the only real read path.
- Tier: 🟡

### 5. [F5] `pkg/state/metrics_history.go:37,45` — exported-but-unreferenced

- Symbol: `state.MetricsRun`, `state.MetricsFile`
- Issue: `unreferenced`
- Diagnosis: both types appear only as return/field types of `LoadMetricsHistory` (F4) and inside its tests. No external code constructs or consumes them.
- Why it matters: same as F4 — envelope without a consumer.
- Evidence: `rg -n 'MetricsRun|MetricsFile' --type go` → declarations + `LoadMetricsHistory` body + tests; nothing in `cmd/fo`.
- Fix: unexport with `LoadMetricsHistory`. If the envelope is intended for a future watch/journal feature, document the intent at the type's godoc instead.
- Tier: 🟡

### 6. [F6] `pkg/report/multiplex.go:66,72` — exported-but-unreferenced

- Symbol: `report.IsDelimiter`, `report.IsDelimiterShape`
- Issue: `unreferenced`
- Diagnosis: both predicates are exported. `IsDelimiterShape` is called only from sibling `HasDelimiter` (line 91) and from `multiplex_test.go`. `IsDelimiter` is called only from `multiplex_test.go`. `cmd/fo/main.go` uses `report.HasDelimiter`, not these.
- Why it matters: the multiplex protocol's actual public contract is `HasDelimiter` + `ParseSections`. The two predicates leak an internal validation distinction (recognized vs. shape-only) that callers neither need nor should branch on.
- Evidence: `rg -n 'IsDelimiter\b|IsDelimiterShape\b' --type go` shows only declarations, the in-package call, and `_test.go` consumers.
- Fix: unexport to `isDelimiter` / `isDelimiterShape`. Tests remain in-package, so no test churn.
- Tier: 🟢

### 7. [F7] `pkg/paint/paint.go:102` — exported-but-unreferenced

- Symbol: `paint.PadRight(s string, width int) string`
- Issue: `unreferenced`
- Diagnosis: `PadRight` is exported but the only external-style usage is in `paint_test.go`. The single production caller is sibling `Columnize` (line 163). `pkg/view/*` callers reach for `paint.PadLeft` but never `paint.PadRight`.
- Why it matters: `PadLeft` and `PadRight` look like a symmetric pair; exporting both implies external callers may need either. Only one is actually consumed.
- Evidence: `rg -n 'PadRight' --type go` → declaration, in-package use in `Columnize`, and tests only.
- Fix: unexport to `padRight` (tests live in `pkg/paint`, so still reachable). Keep `PadLeft` exported — `pkg/view/leaderboard.go:45` does call it.
- Tier: 🟢

### 8. [F8] `pkg/status/status.go:171,179` — exported-but-unreferenced

- Symbol: `status.ViewRow`, `status.Status.ToViewRows()`
- Issue: `unreferenced`
- Diagnosis: `ViewRow`'s godoc says it "mirrors view.StatusRow so pkg/view doesn't need to import pkg/status" — but no `pkg/view` code calls `ToViewRows`. Search shows the type and method are declared and never invoked.
- Why it matters: the contract is justified by an import-direction concern that the codebase doesn't actually exercise. The mirror type is dead weight on the API surface, and the rationale in the godoc misleads readers into thinking `pkg/view` consumes it.
- Evidence: `rg -n 'ToViewRows|status\.ViewRow|\.ViewRow' --type go` returns only the declaration and the doc comment.
- Fix: delete `ViewRow` and `ToViewRows`. If the impedance-bridge becomes necessary later, reintroduce when the first consumer needs it.
- Tier: 🟡

### 9. [F9] `pkg/tally/tally.go:60,64,69` (+ `pkg/metrics/metrics.go:44-46`, `pkg/status/status.go:57-59`) — exported-but-unreferenced

- Symbol: `tally.ErrNoHeader`, `tally.ErrNoRows`, `tally.ErrMalformedRow` (+ identical trios in `pkg/metrics`, `pkg/status`)
- Issue: `unreferenced`
- Diagnosis: each shape package exports three sentinel errors. Across all three packages, the sentinels are only consumed by the package's own `_test.go` files via `errors.Is`. No production code outside the defining package branches on them.
- Why it matters: nine exported error vars total. `cmd/fo` treats parse failures as opaque errors. The sentinels exist for in-package test assertions only — they don't need to be exported. (`pkg/wrapper/wrapleaderboard` follows the same pattern; if any sentinel were to be cross-consumed it'd be one of these, but currently none are.)
- Evidence: `rg -n 'ErrNoHeader|ErrNoRows|ErrMalformedRow' --type go` shows only declarations and same-package test consumers.
- Fix: unexport to `errNoHeader` etc. (tests live in-package so still reachable). Re-export only if/when a non-test caller needs to branch.
- Tier: 🟡

### 10. [F10] `pkg/sarif/types.go:93,101` — exported-but-unreferenced

- Symbol: `(*sarif.Result).Line()`, `(*sarif.Result).Col()`
- Issue: `unreferenced`
- Diagnosis: both methods are called only from `pkg/sarif/toreport.go:45-46` (in-package) and from `pkg/sarif/aggregates_external_test.go`. No package outside `pkg/sarif` reaches into a `Result` to ask for `Line()` / `Col()`.
- Why it matters: borderline — they document the "primary location" rule SARIF leaves implicit. But since `toreport.go` is the only producer, the convenience accessors don't earn their place on the public surface. `FixCommand()` is genuinely cross-consumed (three wrapper test files exercise it) and stays exported.
- Evidence: `rg -nP '\.(Line|Col)\(\)' --type go` → declarations, the in-package toreport call, and one external-style test.
- Fix: unexport to `(*Result).line` / `(*Result).col`. Keep `FixCommand` exported. If publishing the "primary location" semantic deliberately, leave them and add `// Exported as part of the Result accessor API` to the godoc — but the current state advertises an unused contract.
- Tier: 🟢

## Notes

- No embedding leaks found. No struct in the repo embeds a named exported type or `sync.{Mutex,RWMutex,WaitGroup}` anonymously. The one `sync.Mutex` in the repo (`pkg/view/stream_test.go:57`) is a named unexported `mu` field.
- No single-impl interfaces. `pkg/view.ViewSpec` is a closed sum-type (unexported marker method `isViewSpec`), intentional and documented (`pkg/view/view.go:1-20`).
- No receiver-kind mixes. Every multi-method type uses either `*T` or `T` consistently. Types verified: `Builder`, `aggregator`, `parser`, `Status`, `Tally`, `Suppression`, `Result`, `Ruleset`, `TestPackageResult`, `diag`, `jscpd`, `Config`, `UnknownFormatError`, `state`, `unionFind`, `bufioReadCloser`.
- No value-receive-large-struct issues in spot checks; `report.Report` flows by pointer through the renderer and by value only into `pickView`, which returns a small `ViewSpec`.

# solid — SRP/LSP/ISP review (repo scope)

RUN_ID: 7858b3a8ea1b
Scope: project (whole fo repo)
Mode: report

## Overall: 🟢

Zero findings. The codebase is well-factored against SOLID at type+interface
granularity. Notably, the three SRP findings from the prior run (bd775e303d86-solid)
have all been fixed — see "Prior findings, re-verified" below. This is not a
padding-avoidance shrug: the evidence shows the three principles this linter checks
have no live target in the current tree.

## Why zero (evidence)

**LSP — no behavioral interface to violate.** The repo has exactly one production
interface, `ViewSpec` (`pkg/view/view.go:18`), a deliberate closed sum-type marker
with a single unexported method:

```go
type ViewSpec interface {
	isViewSpec()
}
```

The doc comment states the intent: "ViewSpec is a closed sum-type: only this package
can satisfy the unexported isViewSpec marker. Adding a ninth variant means adding a
case to render.go's type switch — by design." Its 8 variants carry no behavioral
contract, so `lsp-stub-impl` and `lsp-precondition-strengthened` cannot fire. The only
`panic(` occurrence in non-test code is a string literal used to detect a
panic-prefixed line (`pkg/view/pickview.go:177`), not a stub implementation.

**ISP — no fat interface, no wide-interface caller.** With `ViewSpec` the sole
interface (a 1-method marker), there is no ≥4-method interface for `isp-fat-interface`,
and no wide-interface parameter for `isp-caller-uses-subset`. `ViewSpec` has 8
production implementers, so `interface-with-one-impl` also does not apply — and it sits
at a deliberate sum-type boundary regardless.

**SRP — types are small and single-concern.** No type carries two distinct concern
clusters. Method counts per receiver top out at 6, and each ≥4-method type is cohesive:
- `aggregator` (`pkg/testjson/parser.go`) — `ProcessEvent`/`Results`/`processEvent`/
  `handleBuildEvent`/`handlePass`/`handleFail`/`handleOutput`/`getOrCreate`/`results`:
  one concern, go-test-event aggregation.
- `diag` (`pkg/wrapper/wrapdiag/diag.go`) — `Convert`/`readAndAdd`/`warnOversize`/
  `addLine`: one concern, line-diagnostic → SARIF conversion.
- `sarif.Builder` (`pkg/sarif/builder.go`) — `AddResult`/`AddResultWithFix`/`Document`/
  `WriteTo`: builder pattern; `WriteTo` is the idiomatic `io.WriterTo` output of the
  thing it builds, not a second concern.

The parser DTOs are now clean: `tally.Tally` (`pkg/tally/tally.go:43`) exposes only
`ToLeaderboard` (a cohesive codec hop, line 116) — no renderer method; `status.Status`
(`pkg/status/status.go:43`) exposes no view-facing codec at all.

**Persistence is not on the domain types.** `pkg/state` keeps storage as package-level
functions (`Load`, `Save`, `Reset`, `Append`, `SaveSnapshot`, `LoadSnapshot`,
`SaveRunLog`, `LoadRunLog`). The domain-shaped types carry only projection/query
methods (`Snapshot.PriorIDs`, `Snapshot.Lookup`, `RunLog.RuleSeries`). This is exactly
the shape `srp-persistence-on-domain` protects. And `report.Report`, the canonical IR,
has zero methods — pure data, as the north-star mandates.

## Prior findings, re-verified (all fixed)

The prior solid run raised three SRP findings. I re-read the cited files at their
absolute paths; all three are resolved in the current tree:

- **F1 `tally.Tally` carried `RenderLLM`** — gone. `pkg/tally/tally.go` now has only
  `ToLeaderboard` on the type; grep for `RenderLLM` in the package returns nothing.
- **F2 `status.Status.ToViewRows` / `ViewRow`** — gone. `pkg/status/status.go` has no
  `ViewRow` type and no `ToViewRows` method.
- **F3 `state.Item.report` dead back-pointer field** — gone. `Item`
  (`pkg/state/diff.go:25`) is now a flat row of exported fields; no `.report` field,
  no reads.

## Notes for the human (not findings)

- `Delta.Inner ViewSpec` (`pkg/view/view.go:127`) is documented as constrained by
  convention to 3 of the 8 variants, enforced by `pickView` rather than the type
  system. This is a data invariant inside a closed sum type, not a SOLID rule
  violation — flagging it would require a near-miss rule-id, which the checklist
  forbids. Surfaced only so the guard's importance is visible.

## Self-reflection

- Read the sole interface, every ≥4-method type, and all three prior findings' cited
  files at absolute paths; evidence quoted verbatim.
- No finding manufactured to fill the cap — the honest count is zero.

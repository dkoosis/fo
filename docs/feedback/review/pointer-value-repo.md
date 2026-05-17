# pointer-value review — repo

RUN_ID: bd775e303d86-pointer-value
Scope: whole repo (`pkg/`, `cmd/`, `internal/`), non-test files
Mode: report

## Summary

The codebase is unusually disciplined about pointer-vs-value. No `[]*T`
collections of small elements, no constructor cargo-culting (every `New*`
returning `*T` is either a mutating builder or has pointer-receiver
methods), no primitive pointer wrappers used to fake absence in user-facing
APIs. The one concrete cluster of findings is the `wrapdiag` package, which
preserved a stale `*flag.FlagSet` shape after the v2 CLI dispatch removed
the FlagSet plumbing — leaving four `*string` fields that the body
dereferences as plain values.

## Findings

### 1. [F1] `pkg/wrapper/wrapdiag/diag.go:35-41` — pointer-wrapper-no-nil-use

**Site:** `wrapdiag.diag` struct (fields `toolName`, `ruleID`, `level`, `version`)
**Issue:** `pointer-wrapper-no-nil-use`
**Size estimate:** each `*string` is 8B; replacing with `string` is 16B per
field — same word count on a 64-bit machine, but removes the indirection,
allocation, and nil-check.

**Diagnosis.** The `diag` struct stores four `*string` fields:

```go
type diag struct {
    toolName *string
    ruleID   *string
    level    *string
    version  *string
    stderr   io.Writer
}
```

The only constructor is `Convert` in `convert.go:27`, which takes plain
values via `DiagOpts` and immediately addresses each field:

```go
d := &diag{
    toolName: &opts.Tool,
    ruleID:   &opts.Rule,
    level:    &opts.Level,
    version:  &opts.Version,
    ...
}
```

The body then unconditionally dereferences `*d.toolName`, `*d.ruleID`,
`*d.level`, `*d.version` at `diag.go:48,51,54,57,107,108`. Nil is not a
meaningful state — the comment on `convert.go:5-9` says "DiagOpts carries
the wrapdiag flags as plain values for the v2 CLI dispatch — bypasses the
*flag.FlagSet ceremony of the plugin path." The `*string` shape is the
fossil of that removed plugin path.

**Why (gates).**
1. Mutation through the pointer? **No** — every use is a deref-read.
2. Caller branches on nil? **No** — `d.toolName == nil` at line 45 is a
   struct-level "not initialized" guard, not nil-as-absent semantics for a
   field; once `Convert` is called via the public API it cannot be nil.
3. Wrong linter (receiver-mix, hugeParam, sentinel)? **No.**
4. Below cap? **Cap is 10; we have one cluster.**

**Evidence (Read-verified).**
- `pkg/wrapper/wrapdiag/diag.go` lines 35-41 (struct), 44-63 (deref sites),
  99-108 (`addLine` derefs).
- `pkg/wrapper/wrapdiag/convert.go` lines 20-35 (sole constructor, comment
  acknowledging the legacy shape).
- `rg '\*string|\*bool|\*int\b' pkg/wrapper/` returns exactly these four
  lines — no other wrapper has the pattern.

**Fix.** Drop the indirection; let `diag` own values.

```go
type diag struct {
    toolName string
    ruleID   string
    level    string
    version  string
    stderr   io.Writer
}

func Convert(r io.Reader, w io.Writer, opts DiagOpts) error {
    if opts.Rule == "" { opts.Rule = "finding" }
    if opts.Level == "" { opts.Level = "warning" }
    d := &diag{
        toolName: opts.Tool,
        ruleID:   opts.Rule,
        level:    opts.Level,
        version:  opts.Version,
        stderr:   opts.Stderr,
    }
    return d.Convert(r, w)
}
```

Then change `*d.toolName` → `d.toolName` (4 sites), drop the
`d.toolName == nil` guard at line 45 (replace with `d.toolName == ""`
which already exists on the next line as `errToolRequired`), and update
`diag.go:36-40` field docs. No call-site changes outside the package —
`diag` is unexported.

**Tier:** 🟡 — one cluster, four fields, isolated to a single unexported
struct. Mechanical fix, no API impact, removes a documented-as-legacy
shape and three or four nil-check branches.

## Surveyed and cleared

For completeness, the following candidates were inspected and **dropped**
per the linter's per-finding gates — not findings.

| Site | Rule considered | Why dropped |
|---|---|---|
| `state/diff.go:176` `classifyFinding(f *report.Finding)` | small-by-pointer | `f` is nil-meaningful (`if f != nil` at `makeItem` line 213). Gate 2. |
| `state/diff.go:32` `Item.report *report.Finding` | small-by-pointer | Same — nil signals "no back-pointer". |
| `state/state.go:93,129,196` `*File` / `Append(*File, Run) *File` | ctor-returns-pointer | `File` carries `[]Run` that grows; pointer for identity across lifecycle. |
| `sarif/builder.go:23` `NewBuilder() *Builder` | ctor-returns-pointer | `Builder` is a fluent mutator; pointer-receiver methods require it. |
| `sarif/builder.go:51,58,92,98` `(b *Builder)` | small-by-pointer | Pointer-receiver mutation — exactly what `*T` is for. |
| `sarif/types.go:93,101,110` `(r *Result)` read-only methods | small-by-pointer | `Result` carries `Locations []Location` + `Fixes []Fix`; not small. |
| `testjson/parser.go:174` `NewAggregator() *Aggregator` | ctor-returns-pointer | Stateful event-consumer with pointer-receiver methods. |
| `testjson/types.go:67,72` `(r *TestPackageResult)` | small-by-pointer | Type holds `FailedTests []FailedTest` + `PanicOutput []string`; not small. |
| `suppress/match.go:12` `NewRuleset() *Ruleset` | ctor-returns-pointer | `Match` is on `*Ruleset` and nil-checks the receiver. |
| `suppress/suppress.go:43` `Suppression.Until *time.Time` | pointer-wrapper | Nil-meaningful ("no expiry"); explicit in `Expired` at line 51. |
| `metrics/metrics.go:83` / `status/status.go:97` / `tally/tally.go:109` `absorbXxxLine(..., headerSeen *bool, lineNo *int)` | pointer-wrapper | Pointers mutate caller state — the pointer doing its job. |
| `view/stream.go:83` `writeSnapshot(..., first *bool, ...)` | pointer-wrapper | Same — mutates caller's "have we emitted yet" flag. |
| `sarif/aggregates.go:17` returns `[]FileIssue` (value) | pointer-slice-small | Already returns value slice. Clean. |
| Whole-repo grep `make([]*` in `pkg/` | pointer-slice-small | Zero hits. Clean. |

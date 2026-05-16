# errors-design — diff review (482cfd360229)

Scope: target file set under cmd/fo, pkg/cluster, pkg/report, pkg/sarif, pkg/scene, pkg/suppress, pkg/testjson, pkg/view, pkg/wrapper/wraparchlint.

**Verdict:** 🟡 — error shapes are mostly aligned (good handle-once discipline; wrappers are CLI boundaries that print-and-return cleanly; one real `errors.As` consumer for `UnknownFormatError`). Main issues: a cluster of exported sentinels in `pkg/scene` and `pkg/suppress` with no `errors.Is` consumers anywhere (P1 sentinel-without-callers), and one mildly redundant wrap. No typed-nil traps, no silent recovers, no boundary leaks (CLI prints errors to stderr; not a network handler).

## Findings

### 1. [F1] `pkg/scene/scene.go:109-114` — sentinel-without-callers

**Issue:** `shape-mismatch` (sentinel-without-callers)

**Evidence:** Five exported sentinels — `ErrNoHeader`, `ErrMalformedAct`, `ErrMalformedActor`, `ErrMalformedExit`, `ErrUnknownAttr` — declared and wrapped at every parse-error site. No `errors.Is(..., scene.Err*)` anywhere in the tree (only one external `errors.Is(err, ErrNoHeader)` exists, and that's in `pkg/tally`, a different package's identically-named sentinel). The exports promise a branchable taxonomy that callers never branch on.

```go
var (
    ErrNoHeader       = errors.New("scene: missing '# fo:scene' header")
    ErrMalformedAct   = errors.New("scene: malformed act header")
    ErrMalformedActor = errors.New("scene: malformed actor line")
    ErrMalformedExit  = errors.New("scene: malformed exit trailer")
    ErrUnknownAttr    = errors.New("scene: unknown header attr")
)
```

**Fix:** Pick one. Either (a) unexport (`errNoHeader`, …) — they're internal-classification flavor only, and `fmt.Errorf("scene: line %d: %w", …)` already gives callers the user-facing message; or (b) keep one exported `ErrParse` and wrap the unexported five with it, so the public API is the single category callers might actually branch on. Five untouched exports is API surface paid for nothing.

---

### 2. [F2] `pkg/suppress/suppress.go:96-102` — sentinel-without-callers

**Issue:** `shape-mismatch` (sentinel-without-callers)

**Evidence:** `ErrMalformedLine`, `ErrMissingRuleID`, `ErrInvalidDate`, `ErrUnknownKey`, `ErrUnclosedQuote` all exported, all wrapped at parse sites. No `errors.Is(..., suppress.Err*)` consumers in the tree. The doc comment claims "callers can errors.Is for category checks" but the only caller, `cmd/fo/suppress.go:applySuppress`, treats any parse error uniformly (notice + continue).

```go
// Sentinel errors. Parse failures wrap one of these so callers can
// errors.Is for category checks.
var (
    ErrMalformedLine = errors.New("suppress: malformed line")
    ErrMissingRuleID = errors.New("suppress: missing rule_id")
    // ...
)
```

**Fix:** Same options as F1. Given .fo/ignore parse errors all flow into a single Notice with the wrapped message verbatim, unexporting these is the honest move. Revisit if a future caller wants to e.g. distinguish "missing date" from "malformed line" for an editor integration.

---

### 3. [F3] `cmd/fo/watch.go:18` — sentinel-without-callers (weak)

**Issue:** `shape-mismatch`

**Evidence:** `errWatchUsage` is unexported (good), but is wrapped in two places (`fmt.Errorf("%w: -source must be …", errWatchUsage)`) and the only consumer (`runWatch`) just prints it. The wrap chain is never inspected via `errors.Is`. Not a public-API problem — flagging as a minor case where a plain `fmt.Errorf("watch: usage: -source must be fs or stdin")` would be equivalent. Low priority.

**Fix:** Leave as-is, or collapse to plain `fmt.Errorf` if the sentinel ever feels like dead weight. Borderline.

---

### 4. [F4] `cmd/fo/watch.go:56` — wrap-redundant (mild)

**Issue:** `wrap-redundant`

**Evidence:** `fmt.Errorf("watch: %w", err)` where `err` is `flag.ErrHelp` or a flag-parse error. The CLI then prints `"fo: %v\n"` with this error. Result on flag error: `fo: watch: flag provided but not defined: -xyz`. The `"watch:"` prefix is informative-ish (tells the user the failure is in `fo watch` arg parsing), but the same context is already implicit from `args[0] == subWatch` dispatch. Borderline; keep if the doubled label is consciously chosen for clarity.

```go
if err := fs.Parse(flagArgs); err != nil {
    return nil, watchOpts{}, fmt.Errorf("watch: %w", err)
}
```

**Fix:** Acceptable. Flag only if the duplicated label ever shows up as `fo: watch: watch:` (it doesn't today). No change required.

---

## Notes (not findings)

- **Good:** `pkg/report.UnknownFormatError` is a typed error with a real `errors.As` consumer in `cmd/fo/main.go:739` that uses it to inject a contextual hint. This is the model the package-scene/suppress sentinels should aspire to (or unexport).
- **Good:** `cmd/fo/main.go:errUnrecognizedInput` and `errTruncatedTestJSON` are unexported sentinels wrapped with `%w`; only the CLI consumes them and there's no claim of cross-package branchability — correct shape for package-internal taxonomy.
- **Good:** Handle-once discipline holds across the diff. Errors flow up to `cmd/fo/main.go` and are printed once at the top of the stack. No `log.Error(err); return err` anywhere in scope.
- **Good:** No `recover()` in any reviewed file. No goroutine panic-conversion to silence.
- **Good:** No typed-nil-as-interface traps. `report.Report` pointers are constructed before return; `*Suppression` is never returned through an `error` interface.
- **Good:** `pkg/sarif/aggregates.go` and `pkg/cluster/*` are pure-compute paths that return no errors — correct, no shape needed.
- **Note:** `pkg/scene/scene.go:117 Parse` wraps `sc.Err()` as `fmt.Errorf("scene: read: %w", err)` — the "scene:" prefix here is genuine context-add (distinguishes IO read failure from semantic parse failure), not redundant.

## Score

| Tier | Result |
|------|--------|
| P1 shape | 🟡 — 2 packages of exported sentinels with no `errors.Is` consumers |
| P1 wrap  | 🟢 — informative; one borderline case |
| P1 handle-once | 🟢 — uniform |
| P2 boundary | 🟢 — CLI-only; stderr is the right sink |
| P2 typed-nil | 🟢 — none |
| P3 recover | 🟢 — none |

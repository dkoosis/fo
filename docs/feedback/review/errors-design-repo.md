# errors-design — fo (repo)

Reviewer: lintbrush `errors-design` (mode: report, scope: project)
Date: 2026-05-16
Module: `github.com/dkoosis/fo`

## Summary

Error design in fo is in good shape overall. Wrapping is consistently `errors.Is`-friendly, no `recover()` exists anywhere in the codebase, and the durability sentinel (`ErrDurabilityDegraded`) is wired through a real caller (`cmd/fo/state.go:39`). The handle-once discipline is excellent: the CLI prints errors via `fmt.Fprintf(stderr, "fo: ...")` and converts to exit codes; library packages return wrapped errors without logging. No typed-nil-as-error traps were found; the one struct error (`UnknownFormatError`) has real `errors.As` callers reading its fields.

The findings below are mostly P1 shape-mismatches around exported sentinels that exist but are never branched on by any caller — they are documentation, not contract. A few low-severity wrap-redundancy nits round it out.

| Tier | Verdict |
|------|---------|
| P1 shape | 🟡 3 exported sentinels with no production `errors.Is` callers |
| P1 wrap | 🟢 informative; one borderline redundancy |
| P1 handle-once | 🟢 uniform — CLI logs, libs return |
| P2 boundary | 🟢 N/A (no network handlers; CLI stderr is the boundary, already prefixed) |
| P2 typed-nil | 🟢 none |
| P3 recover | 🟢 no `recover()` anywhere |

---

### 1. [F1] sentinel-without-callers — `wrapleaderboard.ErrNoRows` exported with no external `errors.Is` callers

- **Site:** `pkg/wrapper/wrapleaderboard/wrapleaderboard.go:35`
- **Issue:** `sentinel-without-callers`
- **Evidence:** `rg -n 'wrapleaderboard\.ErrNoRows'` across the repo finds zero hits outside the package itself. The only `errors.Is(_, ErrNoRows)` callsite is in-package test code (`wrapleaderboard_test.go:64`). `cmd/fo/main.go:1226` handles the wrapper failure with `fmt.Fprintf(stderr, "fo wrap leaderboard: %v\n", err)` and returns exit 2 — it never branches on identity.

```go
// pkg/wrapper/wrapleaderboard/wrapleaderboard.go
var ErrNoRows = errors.New("wrap leaderboard: no rows on stdin")
...
if rows == 0 {
    return ErrNoRows
}
```

- **Fix:** Unexport to `errNoRows`. Exported sentinels are a public-API commitment; this one promises behavior nobody is consuming. Same change applies if you later add a caller that *does* need to distinguish "no rows" from other failures — promote back to `ErrNoRows` then.

---

### 2. [F2] sentinel-without-callers — `status.ErrBadState` exported with no `errors.Is` callers

- **Site:** `pkg/status/status.go:57`
- **Issue:** `sentinel-without-callers`
- **Evidence:** `rg -n 'errors\.Is\(.*ErrBadState'` returns no results. Tests reference `ErrBadState` by equality in a table (`status_test.go:78`), but no production code or test uses `errors.Is`. The sibling sentinels `ErrNoHeader`, `ErrNoRows`, `ErrMalformedRow` *are* branched on (in their own packages' tests via `errors.Is`).

```go
// pkg/status/status.go
ErrBadState     = errors.New("status: bad state token")
...
return "", fmt.Errorf("%w: %q", ErrBadState, tok)
```

- **Fix:** Either (a) unexport to `errBadState` until a real consumer needs it, or (b) update the test at `status_test.go:78` to assert `errors.Is` like the sibling cases — that at least confirms the wrap is intentional. (a) is preferable; an exported sentinel that nobody uses is dead API surface.

---

### 3. [F3] sentinel-without-callers — `sarif.ErrMissingSARIFVersion` exported with no `errors.Is` callers

- **Site:** `pkg/sarif/reader.go:12`
- **Issue:** `sentinel-without-callers`
- **Evidence:** `rg -n 'ErrMissingSARIFVersion'` finds only the declaration and the single return site in `pkg/sarif/reader.go`. No test, no `cmd/fo/main.go` branch. `main.go:584` and `main.go:807` wrap the SARIF parse failure generically (`parsing SARIF: %w`); the caller never asks "was this specifically a missing-version case?"

```go
// pkg/sarif/reader.go
var ErrMissingSARIFVersion = errors.New("missing sarif version")
...
if doc.Version == "" {
    return nil, ErrMissingSARIFVersion
}
```

- **Fix:** Unexport to `errMissingSARIFVersion`. If a future operator wants a precise diagnostic for SARIF version 1.x vs unversioned input, promote back and add the caller in the same change.

---

### 4. [F4] wrap-redundant — `errors.As` branch on `UnknownFormatError` re-wraps with `%w` but adds no prefix

- **Site:** `cmd/fo/main.go:731-734`
- **Issue:** `wrap-redundant` (mild — the *intent* is the hint, but the wrap is structurally odd)
- **Evidence:** The wrap is `fmt.Errorf("%w\nhint: ...", err)` with no leading prefix. Functionally identical to returning `err` plus a separate hint, but it allocates and threads through `%w` only to preserve identity that the caller (`runFo`) doesn't branch on — `runFo` just prints via `fmt.Fprintf(stderr, "fo: %v\n", err)`.

```go
var ufe *report.UnknownFormatError
if errors.As(err, &ufe) {
    return nil, fmt.Errorf(
        "%w\nhint: for raw line-diagnostic text (e.g. 'go vet', 'gofmt'), pipe through 'fo wrap diag --tool <name>' to produce SARIF",
        err,
    )
}
return nil, fmt.Errorf("parsing report sections: %w", err)
```

- **Fix:** Either drop the `%w` (just `fmt.Errorf("%s\nhint: ...", err)`) since no caller does `errors.As(_, &ufe)` after this point — or keep the wrap and add a real prefix (`"parsing report sections: %w\nhint: ..."`) for consistency with the sibling branch on the next line. Current state is mid-transition between the two.

---

### 5. [F5] duplicated unexported sentinel — `errInvalidLevel` declared twice with overlapping semantics

- **Site:** `pkg/sarif/builder.go:12` and `pkg/wrapper/wrapdiag/diag.go:24`
- **Issue:** `sentinel-without-callers` (variant: same logical error, two definitions, neither branched on)
- **Evidence:**

```go
// pkg/sarif/builder.go:12
errInvalidLevel = errors.New("sarif: invalid level; must be error, warning, note, or none")

// pkg/wrapper/wrapdiag/diag.go:24
errInvalidLevel = errors.New("--level: must be error, warning, note, or none")
```

Both are unexported, so they cannot be cross-package consumed even if a caller wanted to. `wrapdiag/diag.go:54` validates the user-supplied `--level` flag value; `sarif/builder.go:47` validates a level fed *programmatically* by the SARIF builder. These describe the same domain concept (valid SARIF level) but as two ships passing.

- **Fix:** Export one canonical `sarif.ErrInvalidLevel` from `pkg/sarif`. Have `wrapdiag` validate its flag by calling `sarif.NormalizeLevel(*d.level)` (or whatever the existing constructor is) and wrap its return. Halves the truth-of-valid-levels and gives a future flag-parser one place to update.

---

### 6. [F6] wrap shape inconsistency — `state.Save` double-`%w` at line 162

- **Site:** `pkg/state/state.go:162`
- **Issue:** wrap quality (informational; not a defect — Go 1.20+ supports multi-`%w` and the test at `state_test.go:142` verifies the `errors.Is(err, ErrDurabilityDegraded)` path)
- **Evidence:**

```go
if err := syncDir(filepath.Dir(path)); err != nil {
    return fmt.Errorf("%w: %w", ErrDurabilityDegraded, err)
}
```

This is correct and idiomatic (both wraps allow `errors.Is` to find either underlying), but worth a comment because it differs from every other wrap site in the same file, which uses the `state: <op>: %w` shape. A reader unfamiliar with multi-`%w` may read this as a typo. The adjacent doc block at line 111 explains the *why* but not the *form*.

- **Fix:** Add a one-line comment at line 161 — `// double-%w: preserve both ErrDurabilityDegraded (for errors.Is) and the underlying fsync error (for diagnostics)`. No code change needed.

---

### 7. [F7] typed-error-fields-unread risk — `report.UnknownFormatError.SectionIndex` and `.Line` not read by callers

- **Site:** `pkg/report/multiplex.go:50-55`
- **Issue:** `typed-error-fields-unread` (partial — Tool and Format *are* read)
- **Evidence:** Only `multiplex_test.go:187` reads fields, and only `Tool` and `Format`. `cmd/fo/main.go:730` does `errors.As` but never inspects fields — it just appends a static hint. `SectionIndex` and `Line` are formatted into `Error()` for human reading but never consulted programmatically.

```go
type UnknownFormatError struct {
    SectionIndex int // 1-based position of the offending section
    Line         string
    Tool         string
    Format       string
}
```

- **Fix:** Keep the struct (it pays for itself with two used fields), but consider whether `SectionIndex` and `Line` need to be exported. If no caller will ever branch on them, lowercase them and let `Error()` format them — the API surface shrinks without losing information. If a renderer test or future LLM-mode consumer is expected to extract them, leave as-is and document the contract.

---

### 8. [F8] wrap-redundant (mild) — `"reading input: %w"` repeated across wrappers with the same shape

- **Site:** `pkg/wrapper/wraparchlint/archlint.go:24`, `pkg/wrapper/wrapjscpd/jscpd.go:34`, `pkg/wrapper/wrapdiag/diag.go:87`
- **Issue:** `wrap-redundant` (very mild — these *are* informative when read in a stderr stream prefixed with `fo wrap <name>:`, but they duplicate context the dispatcher already provides)
- **Evidence:** All three wrappers do `fmt.Errorf("reading input: %w", err)` on stdin failure. The CLI then prints `fo wrap archlint: reading input: <underlying>` — the user already knows they piped to `fo wrap archlint`, so "wrap archlint: reading input" is the load-bearing context, and the inner `"reading input"` is redundant *unless* the underlying error itself ("EOF", "unexpected token") would otherwise be unattributable.

```go
// pkg/wrapper/wraparchlint/archlint.go:24
return fmt.Errorf("reading input: %w", err)
```

- **Fix:** Two options, low priority either way. (a) Drop the prefix and let the CLI's `fo wrap <name>: %v` carry it. (b) Keep but differentiate — `reading SARIF input`, `reading jscpd input` — so a piped-stderr consumer can tell which wrapper a leaked error came from. (a) is the simpler win.

---

### 9. [F9] handle-once observation (positive) — `cmd/fo/state.go:18-50` `attachDiff` exemplary pattern

- **Site:** `cmd/fo/state.go:18-50`
- **Issue:** none — flagging as a positive exemplar
- **Evidence:** This function demonstrates the pattern the rest of the codebase should match when error categorization matters:

```go
if errors.Is(err, state.ErrDurabilityDegraded) {
    fmt.Fprintf(stderr, "fo: state: warning: %v\n", err)
    r.Notices = append(r.Notices,
        fmt.Sprintf("state: durability degraded (%v) ..."))
    return nil
}
fmt.Fprintf(stderr, "fo: state: save: %v\n", err)
r.Notices = append(r.Notices, ...)
return err
```

The sentinel `ErrDurabilityDegraded` is exported *because* a real cross-package caller branches on identity, and the branch produces a different user-visible outcome (warning + Notice vs failure). Compare with F1/F2/F3, where the sentinels lack this justification.

- **Fix:** none. Lift this as the design rubric in `.claude/rules/CLAUDE.md` or similar: "export a sentinel only when a non-test, cross-package caller branches on it and the branch changes user-visible behavior."

---

## Notes for re-runs

- No `recover()` exists in the codebase — P3 phase is fully passing. If a watch-mode background-goroutine path is added later (e.g. fsnotify worker), revisit silent-recover.
- The CLI-as-boundary discipline is uniform: every error is either returned up to `run` (which prints `fo: %v` to stderr and translates to exit 0/1/2) or is logged with the `fo:` prefix at the leaf. There is no network handler returning raw err to a client, so `boundary-leak-to-client` does not apply.
- Test coverage of `errors.Is` is solid for `state` and the hygiene parsers (`tally`, `metrics`, `status` partial); weakest for SARIF.

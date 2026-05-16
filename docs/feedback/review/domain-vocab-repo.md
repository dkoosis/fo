# domain-vocab review — repo

run_id: f62c7fc3af14
scope: project
target: repo
linter: domain-vocab (call-site readability via domain vocabulary)

## Summary

The codebase is overwhelmingly clean against the domain-vocab rule set. **Zero exported functions with multi-bool parameter lists, zero complex inline func types** (all callbacks are single-param `func(TestEvent)` or zero-arg `func()`). Findings concentrate on a handful of internal call sites where adjacent `bool` parameters create positional traps, one `*bool` "first" flag that smuggles state through a signature, and a couple of magic-literal call patterns in scoring.

Tier roll-up:
- P1 bool-trap: 1 yellow (internal funcs runStream/runStreamBatch — noState/stateStrict pair).
- P1 inline-func-type: green.
- P2 magic-literal: 1 yellow (`score.Score(..., 1, ...)`).
- P2 vocab-drift: green (packages cohesive: `state`, `score`, `fingerprint`, `paint`, `view` all use their own terms).

---

### 1. [F1] bool-trap-multi-arg — runStream / runStreamCtx / runStreamBatch pair adjacent `noState, stateStrict bool`

- **Site:** `cmd/fo/main.go` (unexported, internal package)
- **Issue:** bool-trap
- **Signatures:**
  - `func runStream(stdin io.Reader, br *bufio.Reader, stdout io.Writer, t theme.Theme, stateFile string, noState, stateStrict bool, stderr io.Writer) int` (L881)
  - `func runStreamCtx(ctx context.Context, ... stateFile string, noState, stateStrict bool, stderr io.Writer) int` (L890)
  - `func runStreamBatch(stdin io.Reader, br *bufio.Reader, stdout io.Writer, mode, themeName, stateFile string, noState, stateStrict bool, stderr io.Writer) int` (L982)
- **Call-site sample:** `cmd/fo/main.go:250`
  ```go
  return runStream(stdin, br, stdout, resolveTheme(*themeFlag, stdout), *stateFile, *noState, *stateStrict, stderr)
  ```
  Reader sees `..., *stateFile, *noState, *stateStrict, stderr)` — two adjacent dereferenced bool pointers; swap order silently compiles.
- **Fix:** group state-handling flags into a small struct used at the boundary:
  ```go
  type stateOpts struct { Path string; Disabled bool; Strict bool }
  func runStream(... t theme.Theme, state stateOpts, stderr io.Writer) int
  // call: runStream(..., stateOpts{Path: *stateFile, Disabled: *noState, Strict: *stateStrict}, stderr)
  ```
  Bonus: collapses three positional args into one named value at all three call sites.

---

### 2. [F2] bool-trap-via-pointer — `writeSnapshot(..., first *bool, ...)` smuggles state through a signature

- **Site:** `pkg/view/stream.go:65`
- **Issue:** bool-trap (variant: `*bool` used as in/out state flag)
- **Signature:** `func writeSnapshot(w io.Writer, r report.Report, t theme.Theme, width int, first *bool, mode Mode) error`
- **Call-site sample:** `pkg/view/stream.go:56`
  ```go
  if err := writeSnapshot(w, r, t, width, &first, mode); err != nil {
  ```
  `&first` reads as "address of a bool" — purpose only clear by reading the body (`if !*first { ... }; *first = false`).
- **Fix:** lift the "have I written yet?" state into a tiny struct receiver, or return the next-state value:
  ```go
  type snapshotWriter struct { first bool }
  func (s *snapshotWriter) write(w io.Writer, r report.Report, t theme.Theme, width int, mode Mode) error
  ```
  This also localises the mutation and removes the only `*bool` parameter in the package.

---

### 3. [F3] bool-trap-multi-arg (internal) — `internal/lineread` triple of bool helpers all carry `oversize bool`

- **Site:** `internal/lineread/lineread.go`
- **Issue:** bool-trap (low-grade; unexported helpers)
- **Signatures:**
  - `func finishLine(buf, slice []byte, oversize bool) ([]byte, bool, error)` (L39)
  - `func accumulate(buf, slice []byte, oversize bool) ([]byte, bool)` (L49)
  - `func finishOnError(buf, slice []byte, oversize bool, err error) ([]byte, bool, error)` (L56)
- **Call-site sample:** `internal/lineread/lineread.go:30-34`
  ```go
  return finishLine(buf, slice, oversize)
  buf, oversize = accumulate(buf, slice, oversize)
  return finishOnError(buf, slice, oversize, err)
  ```
  Returns also include a bool that is conventionally "the same oversize flag, possibly toggled" — readers must follow the chain to confirm.
- **Fix:** name the type to make intent visible at every site:
  ```go
  type lineFlags struct{ Oversize bool }
  // or simply: type oversize bool
  ```
  Lightest touch: rename one of the two returned bools (currently both are bool with no parameter name in the return tuple) so callers reading `finishLine` see semantic clarity.

---

### 4. [F4] magic-literal-at-call-site — `score.Score(..., 1, pkg.Name)` repeats unnamed `1` for "occurrence count"

- **Site:** `pkg/testjson/toreport.go:40, 50, 62`
- **Issue:** magic-literal-at-call
- **Signature:** `func Score(severityWeight, occurrenceCount int, path string) float64` (`pkg/score/score.go:76`)
- **Call-site sample:**
  ```go
  Score: score.Score(score.SeverityWeightError, 1, pkg.Name) * panicBoost,
  Score: score.Score(score.SeverityWeightError, 1, pkg.Name) * buildErrorBoost,
  Score: score.Score(score.SeverityWeightError, 1, pkg.Name),
  ```
  The first arg is a named constant (good). The second — a bare `1` — is the documented "occurrence count" and reads as a magic number at every site. SARIF path passes a real `n` (`pkg/sarif/toreport.go:51`), so the literal is genuinely unexplained at these three sites.
- **Fix:** add `score.SingleOccurrence = 1` (or `score.OnePerTest`) in `pkg/score/score.go` and use it in `testjson/toreport.go`:
  ```go
  Score: score.Score(score.SeverityWeightError, score.SingleOccurrence, pkg.Name) * panicBoost,
  ```
  Reader now sees the intent without jumping to the signature. Cost: one named constant.

---

### 5. [F5] magic-literal-at-call-site — `paint.Bar(..., t.Icons.Bar, t.Icons.BarEmpty)` is fine; `paint.Bar(value, limit, width, "█", "░")` would not be — preventive note

- **Site:** `pkg/view/leaderboard.go:41` (current usage is correct)
- **Issue:** preventive (no current violation)
- **Signature:** `func Bar(value, limit float64, width int, filled, empty string) string` (`pkg/paint/paint.go:24`)
- **Call-site sample:** `bar := paint.Bar(r.Value, v.Total, bw, t.Icons.Bar, t.Icons.BarEmpty)` — passes theme-resolved glyphs. Good.
- **Fix:** none. Flagged only to note that future call sites must keep using `theme` glyphs rather than inlining rune literals; consider adding a `paint.BarThemed(t theme.Theme, ...)` helper if a non-theme call site appears.

---

## What is NOT a finding

Surveyed, judged clean:

- **Multi-bool exported APIs:** none. `pkg/theme.Default(stdoutIsTTY bool)` is single, self-documenting.
- **Inline func types ≥2 params:** none. All callbacks (`func(TestEvent)`, `func()`, `func(path string, args []string)`) are 0-1 param.
- **Repeated inline func types:** `func(TestEvent)` appears 4× in `pkg/testjson/parser.go` (L45, 79, 131, 156) but is single-param and idiomatic — does not meet the rule threshold (≥2 params OR ≥2 returns). Optional cleanup: `type EventFunc func(TestEvent)`.
- **Package vocabulary drift:** every package's exported surface stays inside its domain (`pkg/state` → `Load/Save/Diff/Classify/Append`; `pkg/fingerprint` → `Fingerprint/NormalizeMessage`; `pkg/score` → `Score/SeverityWeight/FileCentrality`; `pkg/paint` → `Bar/Sparkline/Pad*/Columnize`). No identifiers in foreign vocabularies.

## Disposition

- F1: yellow — internal but called from three sites; struct refactor cheap.
- F2: yellow — `*bool` smell, very local.
- F3: green-yellow — fully internal, low pressure.
- F4: yellow — three call sites, easy named-constant fix.
- F5: green — preventive only.

Total acted findings: **5**. Cap (15) not approached.

# clarity — repo

Scope: project (whole fo repo, all packages) · RUN_ID: 7858b3a8ea1b

## Summary

Swept doc.go presence, test-file presence, the golangci-lint clarity
surface (dupl/globals/inits/interface-bloat/ireturn/arg-limit), oversized
files, and complexity hotspots across all 27 packages (`cmd/fo`,
`internal/*`, `pkg/*`).

- **Documentation** — every package carries a substantive package-level
  godoc comment (on the package clause of its main file, or in `doc.go` for
  `pkg/cluster`). None reference no symbol; none are missing outright.
  `doc.go` as a separate file is the exception rather than the rule here,
  but the substance the rule protects — a reader can tell what the package
  is for in one screen — holds everywhere checked. Not flagged.
- **Tests** — every package has ≥1 `_test.go` file. No package sits at
  zero tests with LOC ≥200.
- **Duplication** — `dupl` (threshold 150) found zero clones repo-wide.
- **Globals** — `gochecknoglobals` fired 11 times; 10 are lookup
  tables/dispatch tables/compiled regex/sentinel errors (the rule's own
  carve-out) or a documented test seam (`pkg/state/state.go:201`
  `syncDir`). One survives scrutiny — below.
- **Init side-effects** — zero `init()` functions anywhere in the repo.
- **Interface bloat / ireturn** — `ireturn` fired 4 times, all against
  `pkg/view.PickView`/`PickViewMode`/`PickViewModeWithExpand`/`pickInner`
  returning `ViewSpec`. `ViewSpec` is the repo's one interface, a
  deliberately closed 8-variant sum type gated by an unexported marker
  (`pkg/view/view.go:18`, "Adding a ninth variant means adding a case to
  render.go's type switch — by design") — the rule's own carve-out for a
  documented architectural boundary. Not flagged.
- **Oversized files** — largest non-test file is `pkg/view/pickview.go` at
  548 LOC, well under the 1000-LOC threshold. No hits.
- **Long param lists** — `revive/argument-limit` (max 6) fired 3 times,
  across 2 files — both below.
- **Complexity hotspots** — 1 hit (`gocognit`); `funlen`/`nestif`/`maintidx`/
  `cyclop` found nothing — below, rolled up per the rule.

## 1. [F1] `pkg/multiplex/multiplex.go:37` — package-globals

**Diagnosis:** `SupportedFormats` is an exported, mutable `[]string`
presented as the authority on which format values fo accepts — but the
actual acceptance logic is a hardcoded regex alternation that doesn't read
from it. The two can drift independently and nothing would catch it.

**Why:** A reader (or a future caller) has a reasonable reason to believe
mutating or extending `SupportedFormats` changes what fo accepts, since its
own doc comment says "the list of format values fo accepts in delimiter
lines" and it's the value quoted back in `UnknownFormatError`'s message. In
fact the delimiter regex hardcodes `(sarif|testjson)` independently — adding
a third format means editing the regex, and `SupportedFormats` has to be
remembered as a second, unenforced edit site. Today they happen to agree;
there's no test or type binding that keeps them that way.

**Evidence** (`pkg/multiplex/multiplex.go:36-47`):
```go
// SupportedFormats is the list of format values fo accepts in delimiter lines.
var SupportedFormats = []string{"sarif", "testjson"}

var (
	delimiterRe = regexp.MustCompile(
		`^--- tool:(\w[\w-]*) format:(sarif|testjson)(?: status:(\w+))? ---$`,
	)
	// delimiterShapeRe matches the delimiter shape with any word for format,
	// so we can distinguish "no delimiter" from "delimiter with unknown format".
	delimiterShapeRe = regexp.MustCompile(
		`^--- tool:(\w[\w-]*) format:([\w-]+)(?: status:(\w+))? ---$`,
	)
)
```

**Fix:** Derive one from the other, or make `SupportedFormats` a `const`-like
unexported source that builds the regex alternation via `strings.Join`, so
adding a format is a one-line edit with a single point of truth. If the
exported var is meant purely as documentation/error-text, say so in the
comment and consider making it unexported (`supportedFormats`) since nothing
outside the package currently needs to read it. → human judgment (small,
no golangci-lint autofix for this shape).

**Tier:** action

## 2. [F2] `pkg/view/stream.go:87` — long-param-list

**Diagnosis:** `handleSnapshot` and `flushStream` both take 7 positional
parameters (`revive/argument-limit`, max 6), and a third sibling,
`writeSnapshot`, takes 6 — the same `(w, r/pendingClean, t, width, first,
mode)` bag threaded through three internal helpers in one file.

**Why:** `RenderStreamMode` calls `handleSnapshot(w, r, t, width, &first,
mode, pendingClean)` and `flushStream(w, pendingClean, t, width, &first,
mode, rendered)` — four of those seven positions (`w`, `t`, `width`, `mode`)
are identical across every call in the file and never vary per call site.
A reader has to hold "which of these seven slots is which" in their head at
each call, and a future edit that reorders two adjacent same-typed
parameters (e.g. swapping `width int` past another `int`) would compile and
silently misbehave.

**Evidence** (`pkg/view/stream.go:87,99`):
```go
func handleSnapshot(w io.Writer, r report.Report, t theme.Theme, width int, first *bool, mode Mode, pending *report.Report) (streamStep, error) {
```
```go
func flushStream(w io.Writer, pendingClean *report.Report, t theme.Theme, width int, first *bool, mode Mode, rendered bool) error {
```

**Fix:** Bundle the call-invariant quad (`w`, `t`, `width`, `mode`) into a
small unexported `renderCtx` struct built once in `RenderStreamMode` and
passed by value to `handleSnapshot`/`flushStream`/`writeSnapshot`; each
keeps its own varying parameters (`r`/`pendingClean`, `first`, `pending`,
`rendered`) alongside it. Shrinks each signature by 3-4 slots and removes
the reorder hazard. → human judgment (internal-only signatures, no
golangci-lint autofix).

**Tier:** action

## 3. [F3] `cmd/fo/main.go:183` — complexity-hotspots-present

**Diagnosis:** 1 complexity hotspot repo-wide: `run` in `cmd/fo/main.go`
trips `gocognit` at 53 (threshold 15) — more than 3x over. `funlen`,
`nestif`, `maintidx`, and `cyclop` found nothing elsewhere in the repo.

**Why:** Rolled-up pointer only, per this linter's contract — the
per-function fix-up belongs to `/review simplify-flow`, not enumerated
here.

**Evidence** (`cmd/fo/main.go:183`):
```go
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case subWrap:
			return runWrap(args[1:], stdin, stdout, stderr)
```
(golangci-lint: `cognitive complexity 53 of func 'run' is high (> 15)`)

**Fix:** → `/review simplify-flow cmd/fo`.

**Tier:** action

## 4. [F4] `pkg/sarif/builder.go:58` — long-param-list

**Diagnosis:** `AddResultWithFix` takes 7 positional parameters
(`ruleID, level, message, file string, line, col int, fixCommand string`),
one over the `revive/argument-limit` threshold of 6.

**Why:** All seven are orthogonal `Result` fields with no natural grouping
narrower than "the whole result," and the method is a public, chainable
Builder API (`AddResult` already exists as the 6-arg convenience wrapper
around it) — the borderline case the rule's own carve-out gestures at
("constructors where each param is required and orthogonal and a config
struct would just rename the fields"). Two same-typed string runs
(`ruleID, level, message, file`) are the concrete risk: a caller reordering
two adjacent string args compiles silently.

**Evidence** (`pkg/sarif/builder.go:51-58`):
```go
func (b *Builder) AddResult(ruleID, level, message, file string, line, col int) *Builder {
	return b.AddResultWithFix(ruleID, level, message, file, line, col, "")
}

// AddResultWithFix is like AddResult but attaches a fix whose description
// text is the shell command (or grep-ready hint) to resolve the finding.
// An empty fixCommand is equivalent to AddResult (no fix attached).
func (b *Builder) AddResultWithFix(ruleID, level, message, file string, line, col int, fixCommand string) *Builder {
```

**Fix:** Low conviction either way — collapsing into a `ResultOpts` struct
would cost the fluent one-line call sites callers already rely on
(`pkg/wrapper/*` all call `AddResult`/`AddResultWithFix` positionally) for
a marginal ergonomics gain. Worth a human look, not an obvious win. →
human judgment.

**Tier:** borderline

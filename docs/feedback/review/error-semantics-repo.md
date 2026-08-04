# error-semantics — repo-wide pass

RUN_ID: 7858b3a8ea1b
Scope: whole fo repo (condensed cross-package pass). Mode: report.

## Summary

Wrap context, log-and-return, panic-in-policy, and commit discards are all clean
repo-wide. The pre-work scans for vacuous/lossy wraps, `log*.(Error|Warn).*err`,
and non-defer commit discards returned **zero** hits; the only `panic(` match is a
string literal in `pkg/view/pickview.go`. Every `_ =` discard is either a
`strings.Builder` write (documented as infallible) or a `defer`/`Close`-position
cleanup owned by `error-fix`.

The single error-semantics smell is **taxonomy that no production caller reads**.
fo declares ~20 exported error sentinels; production code branches on exactly three
(`state.ErrDurabilityDegraded`, `boundread.ErrInputTooLarge`,
`multiplex.UnknownFormatError` via `errors.As`) plus the healthy `kvtok.ErrUnclosedQuote`
/ `ErrStrayQuote` pair (consumed by `pkg/scene` and `pkg/suppress`). The remaining
exported sentinels are checked only by their own package's `_test.go` — no external
package does `errors.Is` on them. The findings below rank that gap by sharpness.

All three findings tag `internal-sentinel-leaked-exported`: the export is a promise,
but the only referents live inside the declaring package (returns + test asserts).

---

### 1. [F1] `pkg/state/state.go:95` `state.ErrVersionSkew` — internal-sentinel-leaked-exported

**Diagnosis:** `ErrVersionSkew` is exported and returned from three load paths
(`state.go:121`, `snapshot.go:117`, `runlog.go:106`), but the sole production consumer
of `state.Load` flattens it into a generic "starting fresh" — while explicitly
branching on its sibling `ErrDurabilityDegraded` two lines of code away.

**Why:** A sentinel earns its export by letting a caller act differently. `attachDiff`
proves the pattern was intended — it does `errors.Is(err, state.ErrDurabilityDegraded)`
and emits a distinct Notice — yet the load-error path treats schema skew, a corrupt
file, and a missing file identically. `ErrVersionSkew` carries no information the plain
message doesn't; either wire a caller that distinguishes skew (e.g. a "reset sidecar"
hint) or unexport it to `errVersionSkew`.

**Evidence:**
`pkg/state/state.go:95` (declaration):
```go
var ErrVersionSkew = errors.New("state: schema version skew")
```
`cmd/fo/state.go:22` (the one production consumer — flattens all load errors):
```go
	prev, err := state.Load(statePath)
	if err != nil {
		fmt.Fprintf(stderr, "fo: state: %v (starting fresh)\n", err)
		prev = nil
	}
```
`cmd/fo/state.go:39` (the sibling that *does* branch, showing intent):
```go
		if errors.Is(err, state.ErrDurabilityDegraded) {
```
The only `errors.Is(_, ErrVersionSkew)` in the module is `pkg/state/state_test.go:44`.

**Fix:** Either add a caller-side branch in `attachDiff` (skew → distinct message/Notice
so a reader knows the sidecar was reset by a schema bump, not lost), or unexport the
sentinel. As a binary with no external importers, unexporting is non-breaking.

**Tier:** action

---

### 2. [F2] `pkg/status/status.go:53` — hygiene-format sentinel family — internal-sentinel-leaked-exported

**Diagnosis:** The four hygiene-format parsers export twelve sentinels
(`status`/`metrics`/`tally`/`wrapleaderboard`, each `ErrNoHeader` `ErrNoRows`
`ErrMalformedRow`, plus `status.ErrBadState`). Every CLI consumer collapses all of
them into one code path: `fmt.Fprintf(stderr, "fo: parsing X: %v"); return 2`. No
production `errors.Is` references any of the twelve; the only checks are in-package tests.

**Why:** The exit-code contract (2 = fo error) legitimately treats every malformed-input
case the same, so the differentiation the sentinels offer is never consumed — and fo is
a binary, so no external importer reads `pkg/status` etc. The export promises a taxonomy
that serves nobody. Collapse each family to `fmt.Errorf` at the call sites (keeping the
message), or unexport the sentinels to internal helpers the tests still reach.

**Evidence:**
`pkg/status/status.go:53-56` (representative declarations; `metrics.go:40-42`,
`tally.go:56/60/65`, `wrapleaderboard.go:37/40` mirror this):
```go
	ErrNoHeader     = errors.New("status: missing '# fo:status' header")
	ErrNoRows       = errors.New("status: no data rows")
	ErrMalformedRow = errors.New("status: malformed row")
	ErrBadState     = errors.New("status: bad state token")
```
`cmd/fo/render.go:141` (consumer — identical flatten in `renderTally`:79, `renderMetrics`:159):
```go
	s, err := status.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing status: %v\n", err)
		return 2
	}
```
A qualified-name search (`status.ErrNoHeader`, etc.) across the repo returns zero hits
outside the declaring packages; the only `errors.Is` checks live in
`status_test.go` / `metrics_test.go` / `tally_test.go` / `wrapleaderboard_test.go`.

**Fix:** Per package, either unexport (`errNoHeader = errors.New(...)`) — tests in the
same package still compile — or drop the sentinels and return `fmt.Errorf` with the same
text. Deferring: if a future library-embed of these parsers is planned, keep them and
add an `// External callers:` doc line documenting the contract.

**Tier:** borderline

---

### 3. [F3] `pkg/multiplex/multiplex.go:34` `ErrNoSections` + `pkg/sarif/reader.go:20` `ErrNestingTooDeep` — internal-sentinel-leaked-exported

**Diagnosis:** Two parser sentinels are exported and returned, but their production
callers wrap them generically and never `errors.Is` on them. `parseMultiplex` carefully
`errors.As`-branches on `UnknownFormatError` yet passes `ErrNoSections` straight into a
generic `"parsing report sections: %w"`; `parseInput` wraps any `sarif.ReadBytes`
failure (including the `ErrNestingTooDeep` depth-bomb guard) as `"parsing SARIF: %w"`.

**Why:** These two are the closest to load-bearing — a caller *could* want to treat
"no sections" or "nesting too deep" specially (different hint, different exit) — which is
why they were exported. But no production site does, and the only `errors.Is` checks are
in `multiplex_test.go` / `reader_test.go`. The export is currently accidental. This is
lower conviction than F1/F2 because a plausible future caller (a richer diagnostic hint
for depth-bomb input) would consume them.

**Evidence:**
`pkg/multiplex/multiplex.go:34` and `pkg/sarif/reader.go:20`:
```go
var ErrNoSections = errors.New("no sections found in report input")
```
```go
var ErrNestingTooDeep = errors.New("sarif nesting too deep")
```
`cmd/fo/parse.go:293` (branches on `UnknownFormatError` but not `ErrNoSections`):
```go
	sections, prelude, err := multiplex.ParseSections(input)
	if err != nil {
		var ufe *multiplex.UnknownFormatError
		if errors.As(err, &ufe) {
			return nil, fmt.Errorf(
				"%w\nhint: for raw line-diagnostic text (e.g. 'go vet', 'gofmt'), pipe through 'fo wrap diag --tool <name>' to produce SARIF",
				err,
			)
		}
		return nil, fmt.Errorf("parsing report sections: %w", err)
	}
```
`cmd/fo/parse.go:154` (`ErrNestingTooDeep` wrapped generically):
```go
		doc, err := sarif.ReadBytes(input)
		if err != nil {
			return nil, fmt.Errorf("parsing SARIF: %w", err)
		}
```

**Fix:** Either add the intended caller branch (e.g. `errors.Is(err, sarif.ErrNestingTooDeep)`
→ a "input rejected: SARIF nesting exceeds guard" hint on stderr; `ErrNoSections` → a
"no `--- tool: ---` delimiters found" hint) and keep the export, or unexport both. Pick
by whether the differentiated user-facing hint is worth writing.

**Tier:** borderline

---

## Notes

- No `vacuous-wrap` / `lossy-wrap` / `redundant-wrap` / `inconsistent-wrap` findings:
  wraps across the repo name their domain (`"parsing SARIF: %w"`, `"commit tx"`-style),
  and no `%s ... err.Error()` / `errors.New(err.Error())` shapes exist.
- No `log-and-return`: production code never logs-then-returns; `cmd/fo` uses
  `fmt.Fprintf(stderr, ...)` at the binary edge (carve-out).
- No `panic-instead-of-error`, no non-defer `silently-discarded-commit`.
- Healthy taxonomy (do not touch): `kvtok.ErrUnclosedQuote`/`ErrStrayQuote` (consumed by
  `pkg/scene`, `pkg/suppress`), `state.ErrDurabilityDegraded` (branched in `cmd/fo/state.go`),
  `boundread.ErrInputTooLarge` (branched in `cmd/fo/main.go:294`),
  `multiplex.UnknownFormatError` (`errors.As` in `cmd/fo/parse.go:295`).

trixi log-skill error-semantics findings 3 --run-id "7858b3a8ea1b"

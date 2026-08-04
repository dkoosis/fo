# errors-design — fo repo (project scope)

RUN_ID: 7858b3a8ea1b

Error design in fo is clean on the mechanical axes: no `fmt.Errorf("%w", err)` no-ops,
no `%!s(MISSING)` missing-arg wraps, no log-and-return double-handling (the code uses
no `log` package at all — errors surface as `Notices` or `Fprintf(stderr, …)`), and
zero `recover()` sites. Wrap prefixes carry real context (format name, line number,
stage). The one recurring smell is **API-surface honesty**: several exported error
sentinels that no production code branches on via `errors.Is` — only the defining
package's own tests reference them. In an application repo (single `cmd/fo` binary over
internal `pkg/*`), an exported sentinel with no in-tree consumer is dead API surface.

Findings ranked most-severe first: 1 action, 4 borderline.

### 1. [F1] `pkg/state/state.go:95` — sentinel-without-callers

**Diagnosis:** `ErrVersionSkew` is exported and returned from three load paths
(`state.go:121`, `snapshot.go:117`, `runlog.go:106`) but no production caller branches
on it. The sole consumer collapses every load error into one generic path.

**Why:** The asymmetry is the tell. Its sibling `ErrDurabilityDegraded` is branched on
in five production sites (`cmd/fo/state.go:39,64,97,118`, `pkg/state/fulllog.go:27`),
so the sentinel-as-contract convention is real here — yet `ErrVersionSkew` is checked
only in `state_test.go`. `Load`'s caller treats a schema-version mismatch identically
to file corruption ("starting fresh"), silently discarding accumulated diff history on
a version bump. Either the sentinel earns a distinct handling (a clearer notice than
generic corruption) or it should be unexported.

**Evidence:** `pkg/state/state.go:95`

```go
var ErrVersionSkew = errors.New("state: schema version skew")
```

The only production consumer, `cmd/fo/state.go:22`, does not distinguish it:

```go
	prev, err := state.Load(statePath)
	if err != nil {
		fmt.Fprintf(stderr, "fo: state: %v (starting fresh)\n", err)
		prev = nil
	}
```

Contrast the same file's sibling handling at `cmd/fo/state.go:39`:

```go
			if errors.Is(err, state.ErrDurabilityDegraded) {
```

**Fix:** Either add an `errors.Is(err, state.ErrVersionSkew)` branch at the `Load`
call site (e.g. a distinct "state schema changed — diff history reset" notice, since
version skew is expected on upgrade and semantically different from corruption), or
unexport it to `errVersionSkew` since nothing outside the package acts on it.

**Tier:** action

### 2. [F2] `pkg/tally/tally.go:56` — sentinel-without-callers (hygiene-format cluster)

**Diagnosis:** The hygiene-format parse packages export a family of error sentinels —
`tally.ErrNoHeader` / `ErrNoRows` / `ErrMalformedRow` (`tally.go:56,60,65`),
`status.ErrNoRows` / `ErrMalformedRow` / `ErrBadState` (`status.go:54,55,56`),
`wrapleaderboard.ErrNoRows` / `ErrMalformedRow` (`wrapleaderboard.go:37,40`) — none of
which any production caller distinguishes. Every `cmd/fo` consumer collapses all parse
errors into one generic message + exit 2.

**Why:** `errors.Is` on these sentinels appears only in each package's own `_test.go`.
The render entry points prove the collapse: `renderTally`/`renderStatus` print
`fo: parsing tally/status: %v` and return 2 for any error, never branching. The
sentinels add public API weight (each is a documented exported var) that buys nothing
callers use. This is one cluster finding, not eight — the same shape repeats across
three packages.

**Evidence:** `cmd/fo/render.go:79`

```go
	t, err := tally.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing tally: %v\n", err)
		return 2
	}
```

Same pattern at `cmd/fo/render.go:141` for status. Defining site, `pkg/tally/tally.go:56`:

```go
var ErrNoHeader = errors.New("tally: missing '# fo:tally' header")
```

**Fix:** These are defensible as a deliberate public parse-error API if external reuse
of `pkg/tally`/`pkg/status` is intended. If not, unexport the row-level ones
(`errMalformedRow`, `errBadState`) that carry no cross-package contract, keeping only
the header/no-rows distinction if a caller ever needs to treat "empty input" as clean
vs. exit 2. At minimum, document the intent so the exported-but-unbranched surface
reads as chosen, not accidental.

**Tier:** borderline

### 3. [F3] `pkg/multiplex/multiplex.go:52` — typed-error-fields-unread

**Diagnosis:** `UnknownFormatError` is a 4-field struct
(`SectionIndex`, `Line`, `Tool`, `Format`) whose fields are consumed only by its own
`Error()` method. The single `errors.As` caller uses the type as a discriminator to
attach a hint and never reads a field — it re-wraps the original `err`.

**Why:** The typed-error ceremony (struct + `Error()`) pays off only when a caller
pulls fields out via `errors.As`. Here the caller wants "is this an unknown-format
error?" — a boolean — not `ufe.Format`. A sentinel + `fmt.Errorf` would carry the same
message and support the same branch with less surface.

**Evidence:** `cmd/fo/parse.go:295` — `errors.As` result `ufe` is bound but no field is read:

```go
		var ufe *multiplex.UnknownFormatError
		if errors.As(err, &ufe) {
			return nil, fmt.Errorf(
				"%w\nhint: for raw line-diagnostic text (e.g. 'go vet', 'gofmt'), pipe through 'fo wrap diag --tool <name>' to produce SARIF",
				err,
			)
		}
```

The fields exist only to format `Error()` (`pkg/multiplex/multiplex.go:59-63`).

**Fix:** Replace the struct with `var ErrUnknownFormat = errors.New("unknown section format")`
and build the detail at the return site via `fmt.Errorf("section %d: %w: format %q for
tool %q", …, ErrUnknownFormat, …)`. The `errors.As` caller becomes `errors.Is`. Keep
the struct only if a caller will genuinely read `Format`/`SectionIndex`.

**Tier:** borderline

### 4. [F4] `pkg/sarif/reader.go:20` — sentinel-without-callers

**Diagnosis:** `ErrNestingTooDeep` (the SARIF depth-bomb guard) is exported and returned
wrapped (`reader.go:72`), but the production caller wraps it generically and never
branches on it.

**Why:** `errors.Is(_, ErrNestingTooDeep)` appears only in `sarif/reader_test.go`.
`cmd/fo/parse.go:154` folds it into `fmt.Errorf("parsing SARIF: %w", err)` alongside
every other parse failure, so the export currently distinguishes nothing at any call
site. A depth-limit rejection and a syntactically broken document produce the same
caller behavior.

**Evidence:** `pkg/sarif/reader.go:20`

```go
var ErrNestingTooDeep = errors.New("sarif nesting too deep")
```

Consumer at `cmd/fo/parse.go:154-156` does not distinguish it:

```go
		doc, err := sarif.ReadBytes(input)
		if err != nil {
			return nil, fmt.Errorf("parsing SARIF: %w", err)
```

**Fix:** Keep exported only if callers should treat "hostile/oversized input" distinctly
from "malformed SARIF" (plausible for a security guard — a different exit path or
message). Otherwise unexport to `errNestingTooDeep`.

**Tier:** borderline

### 5. [F5] `pkg/multiplex/multiplex.go:34` — sentinel-without-callers

**Diagnosis:** `ErrNoSections` is exported and returned (`multiplex.go:161`) but the sole
production caller of `ParseSections` branches only on `UnknownFormatError`, folding
`ErrNoSections` into a generic wrap.

**Why:** `errors.Is(_, ErrNoSections)` is test-only (`multiplex_test.go`).
`parseMultiplex` is reached only after `HasDelimiter` is true, so "no sections found"
is an unusual path; when it fires the caller cannot tell it apart from other section
errors. Exported, unbranched.

**Evidence:** `pkg/multiplex/multiplex.go:34`

```go
var ErrNoSections = errors.New("no sections found in report input")
```

Consumer at `cmd/fo/parse.go:293,302` branches on the sibling typed error but collapses this one:

```go
	sections, prelude, err := multiplex.ParseSections(input)
	if err != nil {
		var ufe *multiplex.UnknownFormatError
		if errors.As(err, &ufe) {
			...
		}
		return nil, fmt.Errorf("parsing report sections: %w", err)
```

**Fix:** Unexport to `errNoSections` unless an external consumer of `pkg/multiplex` is
intended to branch on the empty-input case.

**Tier:** borderline

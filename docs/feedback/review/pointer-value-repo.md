# pointer-value — repo review

Scope: whole repo. Swept every `func F(*T)` / `func F() *T` / pointer-receiver
constructor across `pkg/`, `cmd/fo/`, `internal/` (pre-work regexes +
manual struct-size/mutation check on each hit).

Note: the prior version of this report (RUN_ID bd775e303d86) flagged
`pkg/wrapper/wrapdiag/diag.go` for four stale `*string` fields. Re-checked
this pass — that struct now holds plain `string` fields (`toolName string`,
`ruleID string`, `level string`, `version string`); the fix already landed.
Not re-reported.

Most candidates this pass were disqualified by the per-finding gate before
writing: `report.Report`/`Finding`/`DiffSummary` are genuinely large (>64
bytes, several string fields) so their `*T` plumbing is fine; `*state.File`,
`*state.Diff`, `*suppress.Ruleset`, `*state.Snapshot` all treat `nil` as a
meaningful "absent/empty" state; `sarif.Builder`, `wraparchlinttext.pending`,
`cluster.unionFind`, `scene.parser` are all mutated through the pointer
(builder/accumulator patterns doing their job); `multiplex.UnknownFormatError`
is an `errors.As` sentinel type (out of scope — errors-design); `first *bool`
in `pkg/view/stream.go` is written through (`*first = false`), not read-only.
No `[]*T` pointer-slice-of-small-element pattern exists anywhere in the repo.

One finding survived the gate.

## Findings

### 1. [F1] `pkg/wrapper/wrapjscpd/jscpd.go:24` — ctor-returns-pointer-no-mutation

**Diagnosis:** `jscpd` is a zero-field struct (`struct{}`) whose only method,
`Convert`, uses a pointer receiver purely to exist — there is no state to
read or mutate. `newJscpd()` returns `*jscpd`, and the package-level
`Convert` entry point takes the address of a fresh composite literal on
every call just to invoke it.

**Why:** A zero-size type never needs pointer semantics — there's nothing
behind the pointer to protect from copying, and nothing for the method to
mutate. Pointer receiver plus `&jscpd{}` buys nothing here but ceremony: an
address-of on an empty struct forces it out of the "value passed in
registers" path at the call boundary for a value that carries zero
information. A value receiver on `jscpd{}` would let both call sites
(`newJscpd()`, the package `Convert` shim) pass the value directly with no
behavior change. Contrast with the sibling wrapper `wrapdiag.diag`, which
has 5 real fields and legitimately earns its pointer receiver — this one
doesn't.

**Evidence** (`pkg/wrapper/wrapjscpd/jscpd.go`):
```go
// jscpd converts jscpd JSON to SARIF.
type jscpd struct{}

func newJscpd() *jscpd { return &jscpd{} }

// Convert reads jscpd JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for jscpd reports (typically <1MB).
// Bounded by boundread.DefaultMax to prevent OOM on pathological input (fo-s5x).
func (j *jscpd) Convert(r io.Reader, w io.Writer) error {
```
and the production entry point (`pkg/wrapper/wrapjscpd/convert.go`):
```go
func Convert(r io.Reader, w io.Writer) error {
	return (&jscpd{}).Convert(r, w)
}
```

**Fix:**
```before
type jscpd struct{}

func newJscpd() *jscpd { return &jscpd{} }
```
```after
type jscpd struct{}

func newJscpd() jscpd { return jscpd{} }
```
```before
func (j *jscpd) Convert(r io.Reader, w io.Writer) error {
```
```after
func (j jscpd) Convert(r io.Reader, w io.Writer) error {
```
```before
func Convert(r io.Reader, w io.Writer) error {
	return (&jscpd{}).Convert(r, w)
}
```
```after
func Convert(r io.Reader, w io.Writer) error {
	return jscpd{}.Convert(r, w)
}
```
`jscpd_test.go`'s five `newJscpd().Convert(...)` call sites need no change —
they only invoke a method, indifferent to whether the receiver is a pointer
or value.

**Tier:** borderline — real, but low cost: one avoidable escape per `fo wrap
jscpd` invocation (a CLI subcommand run once per process, not a hot loop).
Worth a drive-by fix, not urgent.

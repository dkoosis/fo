# test-effectiveness — repo (run 7858b3a8ea1b)

Scope: whole repo, 85 `_test.go` files, 484 `Test*` functions. No testify/mock library
in use anywhere (`rg -l 'stretchr/testify'` empty) — every assertion is hand-written
stdlib `if ... { t.Fatalf/Errorf }`, so the evergreen-assertion and mock-misuse rule
families (which target `assert.NotNil`/`mock.On` idioms) have no matching surface here.
Swept for their manual equivalents instead: `if x == nil` after a never-nil
constructor, self-compare, just-assigned-then-checked fields, whole-struct
`reflect.DeepEqual`/`cmp.Diff` where the unit's responsibility is narrower, and
exported symbols with no non-test caller. Also checked the most recently landed
code (`pkg/state/fulllog.go`, `cmd/fo/state.go` — fo-w1f tee-to-log + the
`ErrDurabilityDegraded` fix) since fresh code is the highest-yield place for gaps.

Result: the suite is disciplined — per-field assertions (not whole-struct) in
`pkg/suppress`, `pkg/scene`, `pkg/testjson`; the whole-struct `DeepEqual` uses found
(`pkg/view/invariants_test.go`, `pkg/view/scene_llm_test.go`) are round-trip/purity
checks where "the whole value is unchanged" *is* the unit under test, not an
over-broad shortcut. One finding survives verification.

## 1. [F1] `pkg/sarif/builder.go:92` `Builder.Document` — exported-for-testing-same-package

**Diagnosis:** `Document()` has zero callers outside `pkg/sarif`'s own test files.
Every one of its 7 call sites is a `_test.go` file; `snipe importers sarif.Document`
returns no results, and no production package that imports `pkg/sarif` (archlint,
wraparchlinttext, wrapdiag, wrapjscpd, wrapcoverprofile, `cmd/fo/parse.go`) calls it —
they all go through `WriteTo`.

**Why:** The doc comment frames it as a deliberate escape hatch ("I know what I'm
doing... for tests and inspection") and `Builder`'s own doc says it's "designed... as
an importable library," so this may be intentional forward-looking public API rather
than a pure test-visibility export. That ambiguity is why this is borderline, not
action: if it's truly test-only, a future refactor of `Builder`'s internals could
silently change `Document()`'s shape with only same-package tests as a check —
production code never exercises this path, so a break here would surface as a test
failure with no corresponding user-facing symptom to explain it.

**Evidence** (`pkg/sarif/builder.go:89-94`):
```go
// Document returns the constructed SARIF document without validation.
// Use WriteTo for production output — it validates driver name and levels.
// This method is the "I know what I'm doing" escape hatch for tests and inspection.
func (b *Builder) Document() *Document {
	return b.doc
}
```
Call sites, all test files: `pkg/sarif/builder_test.go:53,64,109,123,134`,
`pkg/sarif/toreport_test.go:125,140`.

**Fix:** If `Document()` is meant as a genuine library-consumer API, leave it and drop
this finding on review — no code change needed. If it exists only to let tests peek at
the built doc, unexport it (tests in `builder_test.go` are already `package sarif` and
can see it fine) and have `toreport_test.go` (external `sarif_test` package) build its
fixture via a small package-internal test helper instead of the public accessor.

**Tier:** borderline — real "test-only export" shape, but a plausible library-API
justification exists that a human should weigh, not a clear-cut smell.

Rule: `exported-for-testing-same-package`

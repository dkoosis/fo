# api-surface — repo review

Scope: project (whole fo repo) · RUN_ID: 7858b3a8ea1b

## Summary

Swept all five sub-checks across `pkg/`, `internal/`, `cmd/`:

- **Embedding leaks** — none. No struct in the codebase embeds an unnamed field
  (verified via struct-body scan across pkg/internal/cmd); no `sync.Mutex`/`sync.RWMutex`
  appears embedded anywhere outside a test helper's named field.
- **Single-implementation interfaces** — the repo defines exactly one interface,
  `pkg/view.ViewSpec` (`pkg/view/view.go:18`). It has 8 implementers behind an
  unexported `isViewSpec()` marker, documented as a deliberately closed sum type
  ("Adding a ninth variant means adding a case to render.go's type switch — by
  design"). This is the textbook justified exception, not premature abstraction —
  not flagged.
- **Receiver-pointer-or-value inconsistency** — none. Cross-checked every
  production type with ≥2 methods; no type mixes pointer and value receivers.
- **Big-struct-by-value in exported signatures** — `report.Report` (~170 bytes)
  passes by value through `PickView`, `RenderReport`, and even a
  `<-chan report.Report` in `RenderStream`. This matches the documented IR
  contract (fo north-star: "Report is the IR... parsers produce it; renderers
  consume it") and slices/pointers inside keep the actual copy cheap — treated
  as the "intentionally passed by value" exception, not flagged.
- **Exported-but-unreferenced** — one clear hit, below.

## 1. [F1] `pkg/testjson/funcresults.go` — exported-but-unreferenced

**Diagnosis:** `FuncResults`, `FuncKey`, `FuncResult`, and `FuncStatus` are
exported from `pkg/testjson` but have zero callers anywhere in the codebase
outside their own unit test.

**Why:** This is a full mini-API (a function plus three exported types) that
isn't wired into the package's actual production path. `pkg/testjson/toreport.go`
— the file that turns parsed events into a `report.Report` — never calls
`FuncResults`; neither does `cmd/fo` or any renderer. It's exported surface
with no live consumer, i.e. either dead code that should be removed, or a
future feature that was scaffolded and never plugged in — either way it's
paying the "every exported name is a contract" cost for zero current benefit.

**Evidence** (`pkg/testjson/funcresults.go:5-29`):
```go
// FuncStatus represents the outcome of a single test function.
type FuncStatus int
...
// FuncKey identifies a test function within a package.
type FuncKey struct {
	Package string // e.g., "github.com/foo/bar"
	Func    string // e.g., "TestBaz"
}

// FuncResult holds the outcome of one test function in one package.
type FuncResult struct {
	Key    FuncKey
	Status FuncStatus
}

// FuncResults processes a TestEvent stream and returns per-function outcomes.
func FuncResults(events []TestEvent) map[FuncKey]FuncResult {
```
Confirmed via repo-wide search: the only call site is
`pkg/testjson/funcresults_test.go:92` (`got := FuncResults(tt.events)`). No
match for `FuncResults(`, `FuncKey`, `FuncResult`, or `FuncStatus` in
`pkg/testjson/toreport.go`, `pkg/testjson/parser.go`, `cmd/fo/*.go`, or any
other package.

**Fix:** Either delete the file (function + three types) if the per-function
breakdown isn't on a near-term roadmap, or wire it into `ToReport`/`ToReportWithMeta`
(`pkg/testjson/toreport.go`) so it has a real caller and the surface earns its
keep. Don't leave it exported-and-orphaned.

**Tier:** action

---

No other findings survived verification. The repo's exported surface is
unusually clean for its size (56 packages, 93 edges per the dependency graph):
one interface total, uniform receiver conventions, and no embedding leaks —
consistent with the deliberate, small-API discipline the north-star doc
describes.

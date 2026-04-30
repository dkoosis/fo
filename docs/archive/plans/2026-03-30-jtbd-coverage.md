# JTBD Coverage — Implementation Plan

**Goal:** Report which Jobs To Be Done have passing tests, which are failing, and which have no coverage — for any Go project using `// Serves: CODE` annotations.

**Architecture:** Two pieces: (1) `pkg/jtbd` — static analysis that maps `// Serves:` comments to test functions, (2) a new pattern type `JTBDCoverage` that fo renders as a coverage matrix. Input is `go test -json`; annotations are extracted from source at report time.

**Note:** This is fo's first subcommand that isn't a pure stdin transformation. Every existing path is `stdin → detect → parse → map → render`. The `jtbd` subcommand introduces a second input source (filesystem scanning) alongside stdin. This is architecturally similar to how `wrap` already breaks the mold — `jtbd` doesn't go through `detect.Sniff` because it has its own input pipeline. The subcommand always operates in **batch mode** regardless of TTY detection — it needs all test results before cross-referencing with annotations.

**Tech Stack:** Go (fo), `go/ast` + `go/token` for source parsing, existing `pkg/testjson` for results.

---

## Context

### Problem

Trixi (and any Go project) can annotate tests with `// Serves: CAP-3, RET-1` to trace test coverage back to user-facing goals. Today, `rg "Serves:"` is the only way to see this. There's no way to answer: "which jobs have passing tests? which are red? which have zero coverage?"

### Annotation Format

```go
// Serves: CAP-3, RET-1
func TestHybridSearch_MergesResults(t *testing.T) {
```

The comment appears on the line(s) immediately before a `func Test*` declaration or a `t.Run(` call. Multiple codes are comma-separated. A test can serve multiple jobs; a job can be served by multiple tests.

### Desired Output

```
JTBD Coverage                           14/28 jobs covered

 Code    Tests  Pass  Fail  Job
 CAP-1       6     6     0  ✓ Interact from wherever
 CAP-3       5     5     0  ✓ Capture without breaking flow
 RET-1       7     7     0  ✓ Surface relevant knowledge
 RET-2       6     6     0  ✓ Filter signal from noise
 EVO-1       8     8     0  ✓ Edge creation
 EVO-3      29    29     0  ✓ Background maintenance
 ...
 CAP-2       0     —     —  · Incorporate incoming info
 PROC-1      0     —     —  · Learn routines
 QS-1        0     —     —  · Ingest metrics
```

---

## Design Decisions

### D1: Source scanning, not test-name convention

Parse `// Serves:` from Go AST rather than requiring codes in test names. Existing convention has 258 annotations — don't break them, build on them.

### D2: Annotation → function association

A `// Serves:` comment is associated with the **next** `func Test*` declaration in the same file via `ast.FuncDecl.Doc` — the standard Go doc-comment group. This requires no blank line between the `// Serves:` comment and the function declaration (standard Go convention). If multiple `// Serves:` lines appear in the doc block, merge their codes.

For `t.Run` subtests: `// Serves:` inside a function body before a `t.Run` call associates with that subtest. This is a stretch goal — function-level is sufficient for v1.

### D3: Job names from a manifest (optional)

If a `jtbd.md` file exists at the project root (or in `docs/`), parse job codes and their short descriptions. Otherwise, report codes without descriptions. The manifest is optional — annotations alone are enough for the coverage report.

### D4: Two entry points

1. **`fo jtbd`** subcommand — reads `go test -json` from stdin, scans source for annotations, renders coverage report. Usage: `go test -json ./... | fo jtbd`
2. **Pattern emission** — when fo processes `go test -json` in normal mode and detects a project with `// Serves:` annotations, it can optionally append a `JTBDCoverage` pattern to the output. (Stretch goal — v1 is the subcommand only.)

### D5: Source root discovery

`fo jtbd` needs to know where to scan for annotations. Options:
- Flag: `fo jtbd --source ./` (explicit — avoids collision with `GOROOT` semantics)
- Convention: look for `go.mod` in cwd or parents
- Both: flag overrides, convention as default

Use both. `go.mod` discovery is standard Go convention.

### D6: Exit code semantics for `fo jtbd`

fo documents exit codes: 0=clean, 1=failures, 2=fo error. For `fo jtbd`:
- **0** = all annotated jobs pass (including if some jobs have zero coverage — uncovered ≠ failing)
- **1** = any job has failing tests
- **2** = fo error (can't find go.mod, can't parse annotations, etc.)

Uncovered jobs are informational, not failures. This matters for CI gating.

### D7: Build-error packages

If `go test -json` reports a build error for a package, tests in that package don't run. Annotations in those files exist but have no corresponding `FuncResult`. Policy: treat as **uncovered** (zero tests), but emit a warning to stderr if the user has `// Serves:` annotations in the failing package. Otherwise this is silent data loss.

---

## Tasks

### Task 0: `pkg/testjson` — Per-function result aggregation

The existing `TestPackageResult` has package-level counters (`Passed`, `Failed`, `FailedTests`), not per-function outcomes. JTBD annotations map to function names, so we need per-function status — specifically pass and skip, not just failures (which `FailedTests` already tracks).

**Overlap note:** The existing aggregator's `outputBuf` already keys by test name internally, and `FailedTests` records failed function names. Task 0 deliberately processes the raw `TestEvent` stream rather than reprocessing `TestPackageResult`, because we need pass/skip status that the existing aggregator discards. Consider whether `outputBuf`'s internal tracking can be exposed instead of re-walking events — but the raw-event approach is simpler and avoids coupling to aggregator internals.

Create `pkg/testjson/funcresults.go`:

```go
// FuncStatus represents the outcome of a single test function.
type FuncStatus int

const (
    FuncPass FuncStatus = iota
    FuncFail
    FuncSkip
)

// FuncKey identifies a test function within a package.
// Using a struct key avoids collisions when TestFoo exists in multiple packages.
type FuncKey struct {
    Package string // e.g., "github.com/foo/bar"
    Func    string // e.g., "TestBaz"
}

// FuncResult holds the outcome of one test function in one package.
type FuncResult struct {
    Key    FuncKey
    Status FuncStatus
}

// FuncResults processes a TestEvent stream and returns per-function
// outcomes keyed by (package, function name).
func FuncResults(events []TestEvent) map[FuncKey]FuncResult
```

Implementation:
1. Iterate TestEvent stream
2. Track `action=pass/fail/skip` per `FuncKey{Package, Test}` pair
3. Only record top-level test functions (no subtests — filter on `/` absence in Test field)
4. Last action wins (a test can emit multiple events)

**Estimated scope:** ~60-80 lines + ~60 lines tests.

### Task 1: `pkg/jtbd` — Annotation scanner

Create `pkg/jtbd/scan.go`:

```go
// Annotation maps a JTBD code to test functions that serve it.
type Annotation struct {
    Code     string   // e.g., "CAP-3"
    Package  string   // e.g., "github.com/foo/bar" (derived from file path relative to go.mod)
    TestFunc string   // e.g., "TestHybridSearch_MergesResults"
    File     string   // relative path
    Line     int      // line of the // Serves: comment
}

// Scan walks Go test files under root, parses // Serves: comments
// from FuncDecl.Doc groups (no blank line between comment and func),
// and returns annotations mapped to their test functions.
func Scan(root string) ([]Annotation, error)
```

Implementation:
1. Walk `*_test.go` files using `filepath.WalkDir`
2. Parse each with `go/parser` (comments + declarations)
3. For each `func Test*` decl, check `FuncDecl.Doc` for `// Serves:` lines
4. Split codes on `,`, trim whitespace
5. Derive package import path via `ModulePath()` helper (see Task 1b)
6. Return flat list of `Annotation`

Edge cases: build-tagged files (parse anyway — annotations still valid), generated files (skip `// Code generated` header), multiple `// Serves:` lines in one doc block (merge codes).

Tests: table-driven with fixtures — files containing known annotations → expected output.

**Estimated scope:** ~120-150 lines + ~100 lines tests (module path logic extracted to Task 1b).

### Task 1b: `pkg/jtbd` — Module path helper

"Derive package import path from file path relative to `go.mod` module path" is ~30 lines of non-trivial path logic with real edge cases. Extract as a dedicated helper with its own tests.

Create `pkg/jtbd/modpath.go`:

```go
// ModulePath finds go.mod at or above root, parses the module directive,
// and returns a function that maps absolute file paths to import paths.
// E.g., if go.mod says "module github.com/foo/bar" and root is /x/bar,
// then /x/bar/pkg/baz/baz_test.go → "github.com/foo/bar/pkg/baz".
func ModulePath(root string) (func(filePath string) string, error)
```

Edge cases requiring tests:
- Nested `go.mod` in monorepos (use nearest `go.mod` above root)
- Symlinks in the path (resolve before computing relative path)
- `go.work` workspaces (out of scope for v1 — document and error clearly)
- Root itself is module root vs. root is a subdirectory

**Estimated scope:** ~40-50 lines + ~60 lines tests.

### Task 2: `pkg/jtbd` — Manifest parser (optional job names)

Create `pkg/jtbd/manifest.go`:

```go
// Job holds a JTBD code and its human-readable description.
type Job struct {
    Code string
    Name string
}

// LoadManifest looks for jtbd.md in root or root/docs,
// parses headings like "### CAP-3: Capture without breaking flow",
// and returns the job list. Returns nil (not error) if no manifest found.
func LoadManifest(root string) []Job
```

Parses any ATX heading (`#` through `######`) containing a code matching `[A-Z]+-[0-9]+` followed by `:` and a description. Examples that match:
- `### CAP-3: Capture without breaking flow`
- `## EVO-1: Edge creation`
- `### **CAP-3**: Capture without breaking flow` (bold codes)

Regex: `^#{1,6}\s+\*{0,2}([A-Z]+-[0-9]+)\*{0,2}:\s+(.+)$`

No YAML — the markdown JTBD doc IS the manifest.

**Estimated scope:** ~50 lines + ~50 lines tests.

### Task 3: `pkg/jtbd` — Coverage assembler

Create `pkg/jtbd/coverage.go`:

```go
// CoverageEntry is one row in the coverage report.
type CoverageEntry struct {
    Code      string
    Name      string // from manifest, or ""
    TestCount int
    Pass      int
    Fail      int
    Skip      int
}

// Assemble cross-references annotations with per-function test results
// and returns a coverage report sorted by code.
func Assemble(annotations []Annotation, results map[testjson.FuncKey]testjson.FuncResult, jobs []Job) []CoverageEntry
```

Logic:
1. Build map: `FuncKey{Package, Func}` → [codes] from annotations
2. Walk `FuncResult` map, look up codes for each `FuncKey` pair
3. Aggregate pass/fail/skip per code
4. Merge with job manifest (add uncovered jobs with zero counts)
5. Sort: covered (by code) then uncovered (by code)
6. Emit warnings for annotations in packages that had build errors (no matching `FuncResult` entries for any test in that package — see D7)

Depends on Task 0 (FuncResults with FuncKey), Task 1 (Annotation with Package field), and Task 1b (module path derivation).

**Estimated scope:** ~80 lines + ~80 lines tests.

### Task 3b: `pkg/mapper` — JTBD coverage mapper

Create `pkg/mapper/jtbd.go`:

```go
// MapJTBDCoverage converts []jtbd.CoverageEntry into a pattern.JTBDCoverage.
// Follows the same convention as MapSARIF and MapTestJSON —
// domain types in, pattern types out.
func MapJTBDCoverage(entries []jtbd.CoverageEntry, totalJobs, coveredJobs int) pattern.JTBDCoverage
```

The subcommand calls through this mapper rather than constructing pattern types inline. This exists to preserve the `parse → map → render` pipeline invariant — even though it's essentially struct-field copying, every other fo path goes through a mapper, and JTBD shouldn't be the exception.

**Estimated scope:** ~30 lines + ~30 lines tests.

### Task 4: `JTBDCoverage` pattern type

Create `pkg/pattern/jtbd.go` — fields defined inline to keep pattern free of domain imports:

```go
// PatternTypeJTBD is the pattern type constant for JTBD coverage.
const PatternTypeJTBD PatternType = "jtbd-coverage"

// JTBDEntry is one row in the JTBD coverage report.
// Defined here (not imported from pkg/jtbd) to preserve
// pattern's zero-dependency invariant. The deliberate duplication
// with jtbd.CoverageEntry is intentional — do not collapse them.
type JTBDEntry struct {
    Code             string
    Name             string
    TestCount        int
    Pass, Fail, Skip int
}

type JTBDCoverage struct {
    Entries     []JTBDEntry
    TotalJobs   int
    CoveredJobs int
}

func (j *JTBDCoverage) Type() PatternType { return PatternTypeJTBD }
```

The mapper (Task 3b) handles `jtbd.CoverageEntry` → `pattern.JTBDEntry` conversion.

Implement rendering in all three renderers:
- **Human:** Type switch on `*pattern.JTBDCoverage` — table with color (green ✓ for covered, dim · for uncovered, red ✗ for failing). This fits cleanly into the existing `renderOne` pattern.
- **JSON:** Auto-serializes via the existing JSON renderer. No special handling needed.
- **LLM:** See Task 4b — requires a renderer refactor.

**Estimated scope:** ~60 lines pattern + ~40 lines human renderer + ~0 JSON renderer.

### Task 4b: LLM renderer — `renderOne` type switch (refactor)

**This is the biggest structural issue in the plan.** The LLM renderer dispatches by `SummaryKind` (`SummaryKindSARIF` / `Test` / `Report`), not by pattern type. It collects summaries and tables by concrete type, then dispatches on `summaries[0].Kind`. A new pattern type that isn't a `Summary` or `TestTable` falls through silently.

Two options:
- **(a) Add a `renderOne`-style type switch** to the LLM renderer, matching the Human renderer's approach. Each pattern type gets its own render case. This is the right move — it makes adding future pattern types straightforward.
- (b) Model JTBD output as a `Summary` + `TestTable` with a new `SummaryKind`. Cheaper but muddies the abstraction — JTBD coverage isn't a test summary.

**Choose (a).** Refactor the LLM renderer to accept pattern types via type switch. The existing `SummaryKind` dispatch continues to work for SARIF and test patterns; `JTBDCoverage` gets its own case that renders the plain text table from the design mockup.

**Estimated scope:** ~60-80 lines refactor + ~30 lines JTBD rendering + ~40 lines tests.

### Task 5: `fo jtbd` subcommand

Add to `cmd/fo/main.go`:

```go
case "jtbd":
    return runJTBD(args[1:], stdin, stdout, stderr)
```

`runJTBD`:
1. Parse flags: `--source` (default: discover from cwd), `--format` (human/llm/json)
2. Discover source root (go.mod walk)
3. Read **all** stdin into buffer (batch mode — no streaming, see architecture note above)
4. Scan annotations (`pkg/jtbd.Scan`)
5. Load manifest (optional, `pkg/jtbd.LoadManifest`)
6. Parse `go test -json` from buffered stdin → `testjson.FuncResults` (Task 0)
7. Assemble coverage (`pkg/jtbd.Assemble`) — warnings to stderr for build-error packages (D7)
8. Map to pattern (`pkg/mapper.MapJTBDCoverage`)
9. Render via existing renderer interface
10. Exit per D6: 0 if all annotated jobs pass or are uncovered, 1 if any job has failures, 2 on fo errors

Usage: `go test -json ./... | fo jtbd`

**Estimated scope:** ~80 lines + CLI integration tests.

---

## Sequence

Tasks 0, 1, 1b, 2, 4, and 4b are independent. Task 3 depends on 0, 1, and 1b. Task 3b depends on 3 and 4. Task 5 wires everything together.

Parallelizable: `[0, 1, 1b, 2, 4, 4b]` → `[3]` → `[3b]` → `[5]`

Note: Task 4b (LLM renderer refactor) is the riskiest task — it touches existing rendering logic. Do it early in the parallel phase so issues surface before integration.

---

## Non-goals

- **No YAML manifest** — markdown headings are the format.
- **No `t.Run` subtest association** in v1 — function-level is sufficient. Can add later.
- **No auto-detection in default `fo` pipeline** in v1 — explicit `fo jtbd` subcommand only.
- **No cross-project aggregation** — one project at a time.
- **No CI integration** — this is a local developer tool. CI can call it, but we don't build CI plumbing.
- **No `go.work` workspace support** in v1 — single-module projects only. Error clearly if detected.
- **No streaming mode** — `fo jtbd` always batches. See architecture note.

---

## Follow-up (out of scope)

- **`make jtbd` target in consumer projects** — e.g., trixi's Makefile: `go test -json ./... 2>/dev/null | fo jtbd --source .`. Trivial once fo ships the subcommand, but belongs in each consumer repo, not this plan.
- **`make jtbd` in fo's own Makefile** — dogfooding. If fo adds `// Serves:` annotations to its own tests, `make jtbd` runs fo on itself. Cheap to add once the subcommand exists.
- **`t.Run` subtest annotation support** — position-based scanning inside function bodies.
- **Auto-detection in default fo pipeline** — emit `JTBDCoverage` pattern when annotations detected.
- **`go.work` workspace support** — multi-module projects.

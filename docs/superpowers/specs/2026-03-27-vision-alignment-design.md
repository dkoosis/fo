# fo Vision Alignment: Wrapper Plugin System & Format Consolidation

**Date:** 2026-03-27
**Status:** Draft

## Vision

fo is a CLI that takes SARIF or go-test-json on stdin and produces output optimized for LLMs (terse, parsable) or humans (tables, color, sparklines).

Two native input formats. Two output audiences. One job.

## Current State

fo has accumulated formats beyond its core two:

| Format | Role | Status after this work |
|--------|------|----------------------|
| SARIF 2.1.0 | Native input | **Keep** |
| go test -json | Native input | **Keep** |
| fo-metrics/v1 | Custom schema for metrics/conformance | **Cut** |
| report (delimiter protocol) | Multiplexer for multi-tool pipelines | **Keep** (sections must be SARIF or go-test-json) |
| jscpd JSON | Tool-specific parser | **Absorbed into wrapper** |
| archlint JSON | Tool-specific parser | **Absorbed into wrapper** |
| text (opaque passthrough) | Report section type | **Cut** |

## Design

### 1. Wrapper Plugin System

Compiled-in Go packages behind a standard interface. Each wrapper converts a tool's native output into SARIF or go-test-json.

```go
// pkg/wrapper/wrapper.go

package wrapper

import "io"

// Format identifies a fo-native output format.
type Format string

const (
    FormatSARIF    Format = "sarif"
    FormatTestJSON Format = "testjson"
)

// Wrapper converts tool-specific output into a fo-native format.
type Wrapper interface {
    // OutputFormat returns the native format this wrapper produces.
    OutputFormat() Format

    // Wrap reads tool output from r, writes fo-native output to w.
    // args contains any flags passed after "fo wrap <name>".
    Wrap(args []string, r io.Reader, w io.Writer) error
}
```

The interface has two methods. `OutputFormat()` lets fo skip re-sniffing and route directly to the right parser. `Wrap()` does the conversion. The wrapper's name is the registry key — no `Name()` method needed.

**Registry:** A flat `registry.go` file with explicit imports. No init() magic.

```go
// pkg/wrapper/registry.go

package wrapper

import (
    "github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
    "github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
    "github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

var registry = map[string]Wrapper{
    "diag":     wrapdiag.New(),
    "jscpd":    wrapjscpd.New(),
    "archlint": wraparchlint.New(),
}

// Get returns the named wrapper, or nil if not found.
func Get(name string) Wrapper {
    return registry[name]
}

// Names returns all registered wrapper names, sorted.
func Names() []string { ... }
```

**Package layout:** Each wrapper is a package under `pkg/wrapper/`:

```
pkg/wrapper/
    wrapper.go        # interface + Format type
    registry.go       # flat map of name -> Wrapper
    wrapdiag/         # line diagnostics (file.go:12:5: msg) -> SARIF
    wrapjscpd/        # jscpd JSON -> SARIF
    wraparchlint/     # go-arch-lint JSON -> SARIF
```

Naming: `wrapdiag` (not `wrapsarif`) because it converts line-based diagnostics *into* SARIF, not wrapping SARIF input. `wrap<toolname>` for tool-specific wrappers avoids collisions with the tool's own parser package.

### 2. Wrapper Dispatch in cmd/fo

`fo wrap <name> [args...]` becomes a generic dispatcher:

```go
func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
    if len(args) == 0 {
        // print available wrappers
        return 2
    }
    w := wrapper.Get(args[0])
    if w == nil {
        fmt.Fprintf(stderr, "fo wrap: unknown wrapper %q\n", args[0])
        // list available wrappers
        return 2
    }
    if err := w.Wrap(args[1:], stdin, stdout); err != nil {
        fmt.Fprintf(stderr, "fo wrap %s: %v\n", args[0], err)
        return 2
    }
    return 0
}
```

`Wrap()` returns `error`, not an exit code. Errors mean "wrapper couldn't do its job" (malformed input, I/O failure). Whether the wrapped output contains findings (exit 1 vs 0) is encoded in the SARIF/testjson output itself — fo's existing `exitCode()` reads patterns to determine that downstream.

The current `runWrapSarif`, `runWrapJscpd`, `runWrapArchlint` functions and their flag parsing move into wrapper packages. `cmd/fo/wrap_jscpd.go`, `cmd/fo/wrap_archlint.go` are deleted. `parseDiagLine` moves to `wrapdiag`.

**CLI rename:** `fo wrap sarif` becomes `fo wrap diag`. Existing usage like `go vet ./... 2>&1 | fo wrap sarif --tool govet` becomes `go vet ./... 2>&1 | fo wrap diag --tool govet`.

### 3. fo-metrics/v1 Elimination

Wrappers that currently emit fo-metrics/v1 will emit SARIF instead:

**jscpd wrapper (before):** jscpd JSON -> fo-metrics/v1 JSON
**jscpd wrapper (after):** jscpd JSON -> SARIF 2.1.0

Each clone becomes a SARIF result:
- `ruleId`: `"code-clone"`
- `level`: `"warning"`
- `message`: `"12 lines duplicated with other_file.go:20-32"`
- `location`: file/line from clone data

**archlint wrapper (before):** go-arch-lint JSON -> fo-metrics/v1 JSON
**archlint wrapper (after):** go-arch-lint JSON -> SARIF 2.1.0

Each violation becomes a SARIF result:
- `ruleId`: `"dependency-violation"`
- `level`: `"error"`
- `message`: `"component → disallowed import"`
- `location`: file from violation data

Both wrappers use `pkg/sarif.Builder` (already exists) to construct SARIF output.

**Output changes:** SARIF-converted clones and violations will render differently than fo-metrics did. They'll flow through `mapper/sarif.go` and appear as file-grouped diagnostic tables (Summary + Leaderboard + TestTable per file) rather than the previous metrics-style output. This is expected — the SARIF rendering path is richer.

### 4. Report Format Simplification

The report delimiter protocol stays as a multiplexer, but its allowed section formats narrow:

**Before:** `sarif | testjson | text | metrics | archlint | jscpd`
**After:** `sarif | testjson`

The delimiter regex becomes:
```
^--- tool:(\w[\w-]*) format:(sarif|testjson)(?: status:(pass|fail))? ---$
```

`status` field is retained — it's useful for SARIF sections where "0 findings" is pass.

Report sections are not recursive. A section's content is raw SARIF or go-test-json, never another delimited report.

`mapper.FromReport` simplifies: only two section handlers (`mapSARIFSection`, `mapTestJSONSection`). The metrics/archlint/jscpd/text handlers are deleted.

Pipeline producers that currently emit `--- tool:jscpd format:jscpd ---` sections would instead pipe through the wrapper first:
```bash
# Before: jscpd emits raw JSON in a report section
# After:  jscpd wraps to SARIF, then appears as a SARIF section in the report
jscpd --reporters json . | fo wrap jscpd  # standalone
```

Or the report producer wraps inline and uses `format:sarif` in the delimiter.

### 5. Package Changes

| Package | Action | Reason |
|---------|--------|--------|
| `internal/fometrics` | **Delete** | fo-metrics/v1 schema eliminated |
| `internal/jscpd` | **Move into `pkg/wrapper/wrapjscpd/`** | Single consumer post-refactor. Parser becomes an internal detail of its wrapper. Still independently testable as a function. |
| `internal/archlint` | **Move into `pkg/wrapper/wraparchlint/`** | Same rationale. |

### 6. Detect Changes

Verified: `detect.go` contains no fo-metrics references. It sniffs for SARIF (`version: "2.1.0"` + `runs` array), GoTestJSON (NDJSON with `Action` field), and Report (delimiter regex). fo-metrics was handled downstream in `mapper/report.go`'s `mapMetricsSection`, not by the top-level sniffer.

No changes to detect.go.

### 7. Mapper Simplification

`pkg/mapper/report.go` is the file with the most changes:

- Delete `mapMetricsSection`, `mapArchLintSection`, `mapJSCPDSection`, `mapTextSection`
- Delete helper functions: `mapMetricsStatus`, `buildMetricsLabel`, `mapDetailSeverity`
- Remove imports: `fometrics`, `jscpd`, `archlint`
- `mapSection` switch reduces to two cases: `sarif` and `testjson`
- Estimated reduction: ~200 LOC removed from report.go

`pkg/mapper/sarif.go` and `pkg/mapper/testjson.go`: unchanged.

### 8. Usage Text & Docs

Update `cmd/fo/main.go` usage string:

- Remove `fo-metrics/v1` from INPUT FORMATS
- Remove `report` from INPUT FORMATS (it's a multiplexer, not a user-facing format)
- `fo wrap` subcommands become dynamic: list registered wrappers
- Remove format-specific wrap subcommand docs, replace with generic pattern
- `fo wrap sarif` examples become `fo wrap diag`

Update `.claude/rules/CLAUDE.md`:

- Remove `internal/fometrics` from package structure
- Add `pkg/wrapper/` to package structure
- Update key design decisions to mention wrapper system

### 9. Pattern Types

No changes. Summary, Leaderboard, TestTable, Sparkline, Comparison, Error all stay. They're format-agnostic visualization primitives — the design is already clean here.

### 10. Exit Code Semantics

Unchanged. Exit 0 = clean, 1 = failures present, 2 = fo error. The logic in `exitCode()` inspects patterns (TestTable fail items, Error patterns), not input formats.

## Test Strategy

Boot.md has mapper tests (34% → 70%) as the next backlog item. This refactor deletes ~200 LOC from `mapper/report.go` — writing tests for code about to be deleted is waste.

**Sequence:** Land this refactor first, then write mapper tests against the surviving code. The mapper backlog item applies to `sarif.go` (160 LOC) and `testjson.go` (208 LOC), which are unchanged by this work and represent the majority of untested mapper logic.

Existing `mapper/report_test.go` (249 LOC) tests will need updating: delete tests for removed section handlers, keep/update tests for the surviving SARIF and testjson section paths.

## What Changes for Users

| Before | After |
|--------|-------|
| `go vet ... \| fo wrap sarif --tool govet` | `go vet ... \| fo wrap diag --tool govet` |
| `jscpd ... \| fo wrap jscpd \| fo` (emits fo-metrics) | `jscpd ... \| fo wrap jscpd \| fo` (emits SARIF) |
| `go-arch-lint ... \| fo wrap archlint \| fo` | Same command, wrapper now emits SARIF |
| `make qa` report with `format:metrics` sections | Report sections use `format:sarif` after wrapping |
| Custom fo-metrics/v1 JSON | Not supported — wrap to SARIF instead |

The jscpd and archlint CLI invocations are identical. The `fo wrap sarif` → `fo wrap diag` rename is the only user-facing command change.

Rendered output for jscpd/archlint will change: findings now appear as SARIF-style diagnostic tables rather than metrics-style output. This is a visual improvement (richer file-grouped display), not a regression.

## Adding a New Wrapper

To add support for a new tool (e.g., `eslint`):

1. Create `pkg/wrapper/wrapeslint/eslint.go`
2. Implement the `Wrapper` interface — parse tool output, emit SARIF via `sarif.Builder`
3. Add one line to `pkg/wrapper/registry.go`
4. Done. `fo wrap eslint` works.

No changes to detect, mapper, render, or any core package.

## Non-Goals

- External/binary plugin discovery (decided: compiled-in only)
- New pattern types for metrics visualization (SARIF covers findings; scalar metrics can be summary items)
- Changes to renderer code (terminal, llm, json renderers are untouched; rendered output for converted tools will differ)
- Changes to streaming mode (go-test-json TTY)

## Risk

**Low.** This is primarily a subtraction (cut fo-metrics, simplify report mapper) with a small structural addition (wrapper interface + registry). The wrapper packages are straightforward moves of existing code with output format changed from fo-metrics to SARIF. `sarif.Builder` already exists and handles the SARIF construction.

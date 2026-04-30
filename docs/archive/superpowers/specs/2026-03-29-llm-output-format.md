# LLM Output Format Spec

**Date:** 2026-03-29
**Scope:** `fo --format llm` output across all three input types (SARIF, testjson, report)

## Problem

The LLM renderer has three code paths (SARIF, go test, report) that each produce inconsistent output: different severity vocabularies (ERR/FAIL/‡), different section styles, different detail formatting. The primary consumer is Claude processing `make report` output in coding workflows. The format should be optimized for Claude to triage, locate, and fix issues with minimal token waste.

## Design Principles

1. **Action-oriented** — file:line is the most important token (maps directly to Edit calls)
2. **Severity-first** — worst problems surface first, one vocabulary everywhere
3. **No noise** — clean tools don't get sections, passing tests are dropped, zero filler
4. **Consistent** — same format regardless of input type, same symbols everywhere
5. **Zero ANSI** — LLM output never contains ANSI escape codes. This is a contract, not an implementation detail.

## Version Preamble

Every LLM output starts with a version line:

```
fo:llm:v1
```

This lets downstream consumers detect format versions. The preamble is always the first line, followed by the triage line.

## Severity Symbols

Three symbols, one vocabulary, all contexts:

| Symbol | Meaning | SARIF level | Test status | Sort priority |
|--------|---------|-------------|-------------|---------------|
| `✗` | Error — fix this | `error` | `fail`, `panic`, build error | 0 (first) |
| `⚠` | Warning — look at this | `warning` | — | 1 |
| `ℹ` | Note — informational | `note` | — | 2 (last) |

`✔` appears only in the triage line to mark clean tools. It is not a finding-level symbol.

No other severity tokens. `ERR`, `WARN`, `FAIL`, `PASS`, `NOTE`, `SKIP` are all replaced by these symbols.

## Format: Report Mode (primary path)

Report mode is the primary consumer (`make report | fo --format llm`). Multiple tools, each with a section delimiter.

### Triage Line

Always present. Always the second line (after version preamble).

```
{N} ✗ {N} ⚠ {N} ℹ | {failing tools} | {passing tools} ✔
```

Examples:
```
4 ✗ 1 ⚠ 0 ℹ | lint test | vet eval dupl vuln arch ✔
0 ✗ 0 ⚠ 0 ℹ | vet lint test eval dupl vuln arch ✔
12 ✗ 0 ⚠ 3 ℹ | lint test vuln | vet eval dupl arch ✔
```

Rules:
- Always show all three counts, even when zero — Claude needs confirmation
- Tool names from report delimiters (`tool:vet` → `vet`)
- A tool is "failing" if it has any `✗` findings or any `Error` pattern
- A tool with only `⚠` or `ℹ` findings is still "passing" (warnings don't fail a tool)
- Failed tools listed first (no symbol — they get sections below)
- Passing tools grouped with `✔`
- Tools listed in report delimiter order within each group
- Omit `ℹ` count from triage when zero: `4 ✗ 1 ⚠ | lint test | ...` (reduce noise)

### Tool Sections

Only tools with findings get a `##` section.

```
## {tool}
```

- No counts in header — Claude can count
- Sections in report delimiter order
- Clean tools never get sections

### Finding Lines

**SARIF-based tools** (vet, lint, vuln):

```
  ✗ {file}:{line}:{col} {rule} — {message}
  ⚠ {file}:{line}:{col} {rule} — {message}
  ℹ {file}:{line}:{col} {rule} — {message}
```

- Severity symbol → file:line:col → rule ID → em dash → message
- One line per finding, no detail lines (the message is the detail)
- If no line/col: `✗ {file} {rule} — {message}`
- Sort: severity desc (✗ → ⚠ → ℹ) → file asc → line asc → rule asc

**Test-based tools** (test, eval):

```
  ✗ {package} {TestName} ({duration})
    {detail line}
    {detail line}
    ...
    ... ({N} more lines)
```

- Severity symbol → package → test name → duration
- Details indented 4 spaces, max 5 lines (`maxDetailLines = 5`), then `... ({N} more lines)`
- Only failed tests shown — passing and skipped tests are dropped entirely
- Sort: panics first → build errors → test failures. Within failures: package asc → test name asc.

**Duplication tool** (dupl):

```
  ⚠ {fileA}:{startA}-{endA} ↔ {fileB}:{startB}-{endB} — {lines} lines
```

**Architecture tool** (arch):

```
  ✗ {from_pkg} → {to_pkg} — {violation description}
```

### Blank-Line Grammar

- One blank line between version preamble and triage line
- One blank line between triage line and first `##` section
- One blank line between `##` sections
- No blank lines between findings within a section
- One blank line between test findings that have detail lines (for readability)
- No trailing blank line at end of output

### Tool Pass/Fail Classification

A tool is classified as **failing** when:
- It has any `✗`-severity finding (SARIF `error`, test `fail`/`panic`), OR
- It has an `Error` pattern (parse error, tool crash — these become visible `✗` entries)

A tool with only `⚠` or `ℹ` findings is **passing** — warnings don't fail a tool. This matches the convention that lint warnings don't break CI.

### All-Pass Output

When every tool passes and no tool has any findings:

```
fo:llm:v1

0 ✗ 0 ⚠ | vet lint test eval dupl vuln arch ✔
```

When tools pass but some have warnings/notes:

```
fo:llm:v1

0 ✗ 2 ⚠ 1 ℹ | vet lint test eval dupl vuln arch ✔

## lint
  ⚠ cmd/server/main.go:44:12 printf — format %d has wrong type arg
  ℹ pkg/util.go:10:1 godot — comment should end with a period
```

### Full Report Example

A complete report with mixed tool types, failures, and clean tools:

```
fo:llm:v1

3 ✗ 1 ⚠ | lint test | vet eval dupl vuln arch ✔

## lint
  ✗ internal/store/store.go:42:5 errcheck — error return of `f.Close` not checked
  ✗ internal/store/store.go:90:6 unused — func `deprecatedHelper` is unused
  ⚠ cmd/server/main.go:44:12 printf — format %d has arg of wrong type

## test
  ✗ pkg/handler TestDeleteUser (0.2s)
    handler_test.go:55: expected 204, got 500
    handler_test.go:56: response body: {"error": "internal"}
```

## Format: Standalone SARIF

When fo receives raw SARIF (not wrapped in report delimiters).

### Triage Line

No tool list — there's only one tool:

```
{N} ✗ {N} ⚠
```

Omit `ℹ` count when zero. Clean: `0 ✗ 0 ⚠`

### Findings

Same format as SARIF findings in report mode, no `##` header needed:

```
fo:llm:v1

4 ✗ 3 ⚠

  ✗ internal/store/store.go:42:5 errcheck — error return not checked
  ✗ internal/store/store.go:78:2 errcheck — error return not checked
  ⚠ cmd/server/main.go:44:12 printf — format %d has wrong type arg
```

## Format: Standalone Go Test

When fo receives raw go test -json (not wrapped in report delimiters).

### Triage Line

Test-specific triage with counts and timing, using symbols:

```
{N} ✗ / {total} tests {N} pkg ({duration})
```

Or when all passing:

```
0 ✗ / {total} tests {N} pkg ({duration})
```

Examples:
```
3 ✗ / 10 tests 2 pkg (1.8s)
0 ✗ / 7 tests 3 pkg (1.0s)
```

### Findings

Same format as test findings in report mode. Only failures shown:

```
fo:llm:v1

3 ✗ / 10 tests 2 pkg (1.8s)

  ✗ pkg/store PANIC
    panic: runtime error: nil pointer dereference
    goroutine 1 [running]:

  ✗ pkg/handler TestCreateUser_DuplicateEmail (0.3s)
    handler_test.go:45: expected error "email already exists", got nil

  ✗ pkg/handler TestDeleteUser_NotFound (0.1s)
    handler_test.go:78: expected 404, got 500
```

All-pass output:

```
fo:llm:v1

0 ✗ / 10 tests 3 pkg (1.0s)
```

One triage line. No package listing.

## Implementation Notes

### What changes in pkg/render/llm.go

- `Render` → prepend `fo:llm:v1\n\n` to all output
- `renderReport` → rewrite: triage line with tool classification, suppress clean tools
- `renderSARIFOutput` → rewrite: new triage line, new finding format, three-tier severity
- `renderTestOutput` → rewrite: new triage line, failures only, drop passing/skipped
- `sarifScope` → replace with triage line builder
- `llmLevelPriority` → update to three tiers (✗=0, ⚠=1, ℹ=2)
- `writeDetails` → update `maxDetailLines` from 3 to 5
- New: `classifyTool` — determines pass/fail from patterns for triage line
- New: severity symbol helper — maps SARIF level / test status to `✗`/`⚠`/`ℹ`

### What doesn't change

- `Renderer` interface
- `JSON` and `Human` renderers (this spec only affects LLM)
- Pattern types and mappers
- Detection and report parsing
- Exit codes (still driven by pattern content, not renderer)

### Constants

```go
const (
    formatVersion  = "fo:llm:v1"
    maxDetailLines = 5
    symError       = "✗"
    symWarning     = "⚠"
    symNote        = "ℹ"
    symPass        = "✔"
)
```

### Test updates

- All LLM tests in `llm_test.go` need updating to match new format
- Test against symbols (`✗`, `⚠`, `ℹ`, `✔`) not words
- Test version preamble is always first line
- Add tests for all-pass cases (triage line only)
- Add tests for standalone vs report mode triage lines
- Add test for tool classification (✗ findings → failing, ⚠-only → passing)
- Add test for three-tier severity sort order
- Add test for detail truncation at 5 lines with overflow indicator
- Add test for zero ANSI output (existing, keep)

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

## Severity Symbols

Three symbols, one vocabulary, all contexts:

| Symbol | Meaning | When |
|--------|---------|------|
| `✗` | Error/failure — fix this | Lint error, test failure, build error, panic |
| `⚠` | Warning — look at this | Lint warning, SARIF note |
| `✔` | Pass/clean — ignore | Clean tools in triage line only |

No other severity tokens. `ERR`, `WARN`, `FAIL`, `PASS`, `NOTE`, `SKIP` are all replaced by these three symbols.

## Format: Report Mode (primary path)

Report mode is the primary consumer (`make report | fo --format llm`). Multiple tools, each with a section delimiter.

### Triage Line

Always first. Always present. One line.

```
{N} ✗ {N} ⚠ | {failing tools} | {passing tools} ✔
```

Examples:
```
4 ✗ 1 ⚠ | lint test | vet eval dupl vuln arch ✔
0 ✗ 0 ⚠ | vet lint test eval dupl vuln arch ✔
12 ✗ 0 ⚠ | lint test vuln | vet eval dupl arch ✔
```

Rules:
- Always show both counts, even when zero — Claude needs confirmation
- Tool names from report delimiters (`tool:vet` → `vet`)
- Failed tools listed first (no symbol — they get sections below)
- Passing tools grouped with `✔`
- Tools listed in report delimiter order within each group

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
  ✗ φ {file}:{line}:{col} {rule} — {message}
  ⚠ φ {file}:{line}:{col} {rule} — {message}
```

- Severity symbol → `φ` file location → rule ID → em dash → message
- One line per finding, no detail lines (the message is the detail)
- If no line/col: `✗ φ {file} {rule} — {message}`
- Sorted: severity desc → file asc → line asc

**Test-based tools** (test, eval):

```
  ✗ φ {package} {TestName} ({duration})
    {detail line}
    {detail line}
    {detail line}
    ... ({N} more lines)
```

- Severity symbol → `φ` package → test name → duration
- Details indented 4 spaces, max 3 lines, overflow indicator
- Only failed tests shown — passing tests are dropped entirely
- Skipped tests are dropped entirely (not actionable)
- Panics rendered as `✗ φ {package} PANIC` with stack trace as detail

**Duplication tool** (dupl):

```
  ⚠ φ {fileA}:{startA}-{endA} ↔ φ {fileB}:{startB}-{endB} — {lines} lines
```

**Architecture tool** (arch):

```
  ✗ φ {from_pkg} → {to_pkg} — {violation description}
```

### All-Pass Output

When every tool passes, the entire output is one line:

```
0 ✗ 0 ⚠ | vet lint test eval dupl vuln arch ✔
```

## Format: Standalone SARIF

When fo receives raw SARIF (not wrapped in report delimiters).

### Triage Line

No tool list — there's only one tool:

```
{N} ✗ {N} ⚠
```

Clean: `0 ✗ 0 ⚠`

### Findings

Same format as SARIF findings in report mode, no `##` header needed:

```
4 ✗ 3 ⚠

  ✗ φ internal/store/store.go:42:5 errcheck — error return not checked
  ✗ φ internal/store/store.go:78:2 errcheck — error return not checked
  ⚠ φ cmd/server/main.go:44:12 printf — format %d has wrong type arg
```

## Format: Standalone Go Test

When fo receives raw go test -json (not wrapped in report delimiters).

### Triage Line

Test-specific triage with counts and timing:

```
FAIL {N}/{total} tests {N} pkg ({duration})
```

Or when passing:

```
PASS {total} tests {N} pkg ({duration})
```

Uses words `FAIL`/`PASS` here instead of symbols because this is the triage line, not a finding — and `✗ 3/10 tests` reads oddly.

### Findings

Same format as test findings in report mode. Only failures shown:

```
FAIL 3/10 tests 2 pkg (1.8s)

  ✗ φ pkg/store PANIC
    panic: runtime error: nil pointer dereference
    goroutine 1 [running]:

  ✗ φ pkg/handler TestCreateUser_DuplicateEmail (0.3s)
    handler_test.go:45: expected error "email already exists", got nil

  ✗ φ pkg/handler TestDeleteUser_NotFound (0.1s)
    handler_test.go:78: expected 404, got 500
```

All-pass output:

```
PASS 10 tests 3 pkg (1.0s)
```

One line. No package listing — Claude doesn't need to see what passed.

## Implementation Notes

### What changes in pkg/render/llm.go

- `renderReport` → rewrite: triage line, tool grouping, suppress clean tools
- `renderSARIFOutput` → rewrite: new triage line, new finding format
- `renderTestOutput` → rewrite: new triage line, failures only, drop passing
- `sarifScope` → replace with triage line builder
- Severity mapping: SARIF `error` → `✗`, `warning`/`note` → `⚠`
- Test mapping: `fail`/`panic` → `✗`, drop `pass`/`skip`
- `writeDetails` → keep, already does 3-line truncation with overflow

### What doesn't change

- `Renderer` interface
- `JSON` and `Human` renderers (this spec only affects LLM)
- Pattern types and mappers
- Detection and report parsing
- Exit codes (still driven by pattern content, not renderer)

### Test updates

- All LLM tests in `llm_test.go` need updating to match new format
- Test against symbols (`✗`, `⚠`, `✔`) not words
- Add tests for all-pass cases (should be one line)
- Add tests for standalone vs report mode triage lines

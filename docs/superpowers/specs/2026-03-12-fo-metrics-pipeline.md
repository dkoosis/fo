# fo-metrics: A Schema for Scalar Tool Output

## Two Problems, One Schema

fo handles two of the three kinds of build tool output well: location-based findings (SARIF) and test pass/fail streams (testjson). The third kind — scalar metrics, conformance checks, and summary statistics — has no schema. Tools like jscpd, go-arch-lint, and eval either get shoehorned into testjson (losing their semantics) or passed as opaque blobs (losing rendering entirely).

fo-metrics is a lightweight JSON schema that closes this gap. It's useful on its own — any tool that produces metrics can emit it, and fo can render it — independent of any particular CI pipeline.

The secondary problem is that trixi's `make qa` currently works around the gap with grep hacks and opaque passthrough. Once fo-metrics exists, the Makefile can be cleaned up. That's a separate, downstream effort.

### Existing State in Code

`pkg/mapper/report.go` already routes `format:metrics`, `format:archlint`, and `format:jscpd` hints to dedicated mapper functions. `internal/detect/` recognizes the `--- tool:X format:Y ---` delimiter as `Report` format. This spec formalizes and completes that partially-implemented pathway — the schema, transformer contract, pattern mapping, and error handling are what's missing.

## fo's Three Input Formats

```
tool → [transformer?] → fo-compatible format → fo → terminal | llm | json
```

| Format | What it represents | Examples |
|--------|--------------------|----------|
| SARIF | Findings at file:line | go vet, golangci-lint, govulncheck |
| testjson | Test pass/fail/skip with timing | go test -json |
| **fo-metrics** | Scalar metrics, conformance, summaries | eval, jscpd, go-arch-lint, any custom tool |

testjson is a Go ecosystem standard. fo already handles it well. Wrapping it in a canonical JSON adds a translation layer for zero benefit. It stays as-is.

## Current State (trixi's `make qa`)

This table shows how the schema applies to trixi's pipeline, but fo-metrics is not coupled to it.

| # | Tool | Native Output | fo Sees Today | With fo-metrics |
|---|------|---------------|---------------|-----------------|
| 1 | go vet | line diagnostics | SARIF (via `fo wrap sarif`) | no change |
| 2 | golangci-lint | SARIF | SARIF | no change |
| 3 | go test | testjson | testjson | no change |
| 4 | govulncheck | SARIF | SARIF | no change |
| 5 | eval | testjson (wrong) | testjson (metrics buried) | fo-metrics (direct — we own the code) |
| 6 | jscpd | jscpd JSON | opaque blob | fo-metrics (via `fo wrap jscpd`) |
| 7 | go-arch-lint | custom JSON | opaque blob | fo-metrics (via `fo wrap archlint`) |

## fo-metrics Schema

A JSON object emitted per tool. One object per `--- tool:X format:metrics ---` section.

```json
{
  "schema": "fo-metrics/v1",
  "tool": "eval",
  "status": "pass",
  "metrics": [
    {"name": "MRR",     "value": 0.983, "threshold": 0.950, "unit": null,  "direction": "higher_is_better"},
    {"name": "P@5",     "value": 0.227, "threshold": null,  "unit": null,  "direction": "higher_is_better"},
    {"name": "NDCG@5",  "value": 0.961, "threshold": 0.900, "unit": null,  "direction": "higher_is_better"},
    {"name": "FPR",     "value": 0.000, "threshold": 0.050, "unit": null,  "direction": "lower_is_better"}
  ],
  "summary": "86 queries, 0 regressions",
  "details": []
}
```

### Field Definitions

| Field | Type | Required | Default | Purpose |
|-------|------|----------|---------|---------|
| `schema` | string | yes | — | Always `"fo-metrics/v1"`. fo rejects unknown major versions, warns on unknown minor |
| `tool` | string | yes | — | Tool name (matches `--- tool:X` delimiter) |
| `status` | enum | yes | — | `"pass"`, `"fail"`, `"warn"`. **Authoritative** — takes precedence over per-metric threshold checks. Rationale: the emitting tool may have context fo lacks (e.g., acceptable regressions, waived failures) |
| `metrics` | array | yes | — | Metric entries (may be empty) |
| `metrics[].name` | string | yes | — | Display name |
| `metrics[].value` | number | yes | — | Current value |
| `metrics[].threshold` | number\|null | no | `null` | Gate value. null = informational only. Used by fo for color hints (green/red) but does **not** override top-level `status` |
| `metrics[].unit` | string\|null | no | `null` | Display suffix (e.g., `"%"`, `"s"`, `"ms"`). Renderer appends to formatted value. null = no suffix |
| `metrics[].direction` | enum | no | `"higher_is_better"` | `"higher_is_better"` or `"lower_is_better"`. Tells fo how to color deltas and evaluate thresholds |
| `summary` | string | no | `null` | One-line human summary |
| `details` | array | no | `[]` | Drill-down items. See contract below |

### details Array

Itemized findings beyond the top-level metrics. Each entry is a flat object. fo **does** interpret a minimum field set for rendering:

| Field | Type | Required | Used by fo |
|-------|------|----------|------------|
| `message` | string | **yes** | Rendered as the line item text |
| `file` | string | no | Shown as location prefix in terminal mode |
| `line` | number | no | Appended to file as `file:line` |
| `severity` | enum | no | Colors the entry: `"error"` → red, `"warn"` → yellow, else default |
| `category` | string | no | Used for grouping in terminal mode (entries grouped by category, then listed) |

Additional fields are preserved in JSON mode output but ignored by terminal and LLM renderers.

```json
"details": [
  {"message": "clone: store/save.go:40-62 ↔ store/update.go:18-40", "file": "store/save.go", "line": 40, "severity": "warn"},
  {"message": "dependency violation: search/ → embedder/", "category": "arch"}
]
```

### Version Handling

fo checks the `schema` field prefix:
- `fo-metrics/v1` or `fo-metrics/v1.x` — accepted, unknown minor fields ignored
- `fo-metrics/v2` or higher — reject with Error pattern: `"unsupported schema: fo-metrics/v2"`
- Missing or malformed — reject with Error pattern

## Pattern Mapping

fo-metrics input maps to existing `pattern.Pattern` types via `pkg/mapper/`:

| fo-metrics field | Pattern type | When |
|------------------|-------------|------|
| Top-level status + metrics | **Summary** | Always. One SummaryItem per metric, plus status as Kind (pass→KindSuccess, fail→KindError, warn→KindWarning) |
| `details` array | **TestTable** | When `details` is non-empty. Each detail becomes a TestTableItem. Source set to tool name. Status derived from `severity` field |
| metrics with thresholds | (color hints on Summary) | Threshold-breaching metrics get KindWarning/KindError styling |

No new pattern type is needed. The mapper constructs Summary (always) + TestTable (when details present), consistent with how FromSARIF and FromTestJSON already work.

## Transformers

### Where They Live

Transformers are `fo wrap <format>` subcommands. `fo wrap sarif` already exists as the pattern. Add:

- `fo wrap jscpd` — reads jscpd JSON from stdin, writes fo-metrics JSON to stdout
- `fo wrap archlint` — reads go-arch-lint JSON from stdin, writes fo-metrics JSON to stdout

For eval: no transformer needed. Trixi's eval harness emits fo-metrics directly via a new `WriteMetrics` method on `eval.Report`.

### Transformer Contract

- Reads stdin (tool's native JSON)
- Writes fo-metrics JSON to stdout (single JSON object, no trailing newline noise)
- Exit 0 on success, exit 1 on parse failure
- On exit 1: write a human-readable error message to **stderr**
- Stateless, no flags beyond `fo wrap <format>`

### Transformer Failure Handling

When a transformer exits non-zero or produces unparseable JSON, fo:
1. Emits an `Error` pattern for that tool section (source = tool name, message = stderr content or "failed to parse metrics")
2. Continues processing remaining report sections
3. The tool's section contributes a failure to the overall exit code (exit 1)

## fo Rendering

fo already renders SARIF and testjson differently per output mode. Add fo-metrics rendering:

### Terminal Mode

```
eval          pass   MRR=0.983  P@5=0.227  NDCG@5=0.961  FPR=0.000
                     86 queries, 0 regressions
dupl          pass   clones=12  duplication=3.2%
arch-lint     pass   violations=0
```

Metrics with thresholds: green if passing, red if failing. `direction` field determines which way is good. Metrics without thresholds render in default/muted color.

When `details` is non-empty and status is `fail` or `warn`, detail items render below the metrics line, indented, grouped by `category` if present.

### LLM Mode

```
## eval (pass)
MRR=0.983 P@5=0.227 NDCG@5=0.961 FPR=0.000
86 queries, 0 regressions
```

Terse, no ANSI. Details included only if status is `fail` or `warn`.

### JSON Mode

Pass through the fo-metrics object as-is within the structured output.

## Appendix: Makefile Changes (Trixi)

Downstream of the schema work. After fo-metrics ships, REPORT_CMD becomes:

```makefile
REPORT_CMD = \
    echo '--- tool:vet format:sarif ---'; \
    go vet ./... 2>&1 | fo wrap sarif --tool govet; echo; \
    echo '--- tool:lint format:sarif ---'; \
    golangci-lint run --output.sarif.path=/dev/stdout --output.text.path=/dev/null ./... 2>/dev/null | head -1; echo; \
    echo '--- tool:test format:testjson ---'; \
    go test -json -race -timeout=5m -count=1 ./... 2>&1; echo; \
    echo '--- tool:eval format:metrics ---'; \
    go test -run TestEvalMetrics -count=1 ./internal/eval/ 2>/dev/null | grep '^{'; echo; \
    echo '--- tool:dupl format:metrics ---'; \
    TMP_JSCPD=$$(mktemp -d); jscpd . --silent --reporters json --output $$TMP_JSCPD >/dev/null 2>&1; cat $$TMP_JSCPD/jscpd-report.json | fo wrap jscpd; echo; rm -rf $$TMP_JSCPD; \
    echo '--- tool:vuln format:sarif ---'; \
    TMP_VULN=$$(mktemp); govulncheck -format sarif ./... >$$TMP_VULN 2>/dev/null; cat $$TMP_VULN; rm -f $$TMP_VULN; echo; \
    echo '--- tool:arch format:metrics ---'; \
    go-arch-lint check --json 2>/dev/null | fo wrap archlint
```

Changes from current:
- eval: `format:testjson` → `format:metrics`, new `TestEvalMetrics` that emits fo-metrics JSON directly. **`2>/dev/null | grep '^{'`** filters go test framework noise — only lines starting with `{` (the fo-metrics JSON) pass through
- dupl: `format:jscpd` → `format:metrics`, piped through `fo wrap jscpd`
- arch: `format:archlint` → `format:metrics`, piped through `fo wrap archlint`
- `make qa` post-step grep removed (fo renders the eval summary natively)

**Note:** The `grep '^{'` approach for eval is a pragmatic stopgap. A cleaner alternative is having `TestEvalMetrics` write fo-metrics to a tempfile and `cat` it, but the grep approach is simpler and sufficient given fo-metrics output is a single JSON object on one line.

## Implementation Sequence

### Part 1 — fo-metrics as a standalone capability (fo repo)

This is the schema itself. Shippable and testable without any Makefile changes.

**1a. Define fo-metrics types, parser, and mapper**

Add `format:metrics` handling to fo's report parser. Define the mapper function in `pkg/mapper/` that converts fo-metrics JSON → Summary + optional TestTable patterns. Terminal/LLM/JSON output handlers. Test with fixture JSON files — no live tools needed.

**1b. Add `fo wrap jscpd` and `fo wrap archlint`**

Two small transformers. Each ~50-100 lines. Map native JSON fields to fo-metrics schema. Test with captured tool output fixtures.

At this point, `echo '{"schema":"fo-metrics/v1",...}' | fo` works. The schema is real and usable by any tool.

### Part 2 — trixi adoption (parallelizable with Part 1)

**2a. Add `eval.Report.WriteMetrics()` (trixi repo)**

New method on `eval.Report` that emits fo-metrics JSON. Add `TestEvalMetrics` test entry point that runs eval and outputs fo-metrics to stdout.

### Part 3 — pipeline integration (depends on Parts 1 + 2)

**3a. Update Makefile** — switch eval/dupl/arch sections to `format:metrics` with appropriate piping.

**3b. Remove eval post-step hack** — delete the `grep '^eval:'` line from `make qa`.

**3c. End-to-end validation** — run `make qa` and verify all 7 tools render correctly in terminal, LLM, and JSON modes.

## Scope Boundaries

- **Part 1 (fo-metrics schema) is the primary deliverable.** It stands alone as a capability — any tool that emits fo-metrics JSON gets proper rendering in terminal, LLM, and JSON modes.
- Parts 2-3 (trixi adoption, Makefile rewiring) are downstream consumers. They validate the schema against real tools but are not required for fo-metrics to ship.
- No changes to eval query sets, search tuning, or metric definitions
- No changes to SARIF or testjson handling
- fo-metrics is additive — fo continues to handle unknown format hints gracefully
- The `details` array has a minimum renderable contract (see above) but tools may include extra fields

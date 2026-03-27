# Solution Fitness Review — 2026-03-27

Scope: `fo` as a build-output renderer (SARIF, `go test -json`, report sections) with terminal/LLM/JSON output.

This review focuses on **solution fitness**, not code quality.

## 1) Major subsystem map

| Subsystem | Problem solved | Current approach |
|---|---|---|
| Input format detection | Determine whether stdin is SARIF, `go test -json`, or report format | Byte sniffing + lightweight JSON probes in `internal/detect` |
| Parsing layer | Convert raw input to typed documents/events | Full-document parse for SARIF/report and line parser for `go test -json` |
| Mapping layer | Convert parsed data to semantic patterns | Format-specific mappers produce `Summary`, `Leaderboard`, `TestTable`, `Error` |
| Rendering layer | Emit terminal/LLM/JSON output | Renderer dispatch per pattern type + theme abstraction |
| Streaming test UX | Real-time progress for `go test -json` on TTY | Event-driven stream state machine + footer redraw |
| Multi-tool report ingestion | Aggregate heterogeneous tool outputs into one dashboard | Delimiter-based section format with per-section parser dispatch |

## 2) Fitness findings

### Finding A

| Field | Value |
|---|---|
| Subsystem | Batch execution path (`cmd/fo` main pipeline) |
| Category | Wrong Strategy |
| Current approach | For non-stream mode, reads all stdin into memory before parse/map/render (`io.ReadAll`), including `go test -json` when stdout is piped. |
| Alternative | Add true streaming aggregation for `go test -json` in non-TTY modes and incremental section parsing for report input (process section-by-section, emit/accumulate summaries). |
| Impact | **High** — scales memory and latency with full input size; at 10×-100× CI logs this increases peak RSS and delays first useful output. |
| Intentionality | **deliberate-stale** — behavior is documented (“Reads all input from stdin”), but this looks optimized for simplicity rather than current/future large CI traces. |
| Recommendation | **investigate** |

### Finding B

| Field | Value |
|---|---|
| Subsystem | Report parser (`internal/report`) |
| Category | Wrong Algorithm |
| Current approach | Splits entire payload into lines (`bytes.Split`) and constructs section content in memory, effectively materializing all lines and section buffers simultaneously. |
| Alternative | Streaming line scanner (`bufio.Scanner` or `bufio.Reader`) that emits sections as they close; keeps only active section buffer + summary state. |
| Impact | **Medium-High** — unnecessary memory amplification for long reports with many sections; can become a bottleneck before rendering starts. |
| Intentionality | **accidental** — no evidence of an explicit tradeoff for memory vs simplicity in docs/ADR. |
| Recommendation | **change** |

### Finding C

| Field | Value |
|---|---|
| Subsystem | Multi-tool normalization strategy |
| Category | Wrong Abstraction Level |
| Current approach | Supports several bespoke adapters/formats (`sarif`, `testjson`, `metrics`, `archlint`, `jscpd`, `text`) and a custom report delimiter grammar. |
| Alternative | Standardize more tools onto SARIF (or SARIF-like envelope) at ingest boundaries, reducing parser/mapper surface and custom format contracts. |
| Impact | **Medium** — current approach increases long-term integration cost (new tool = new parser/mapper path), though short-term flexibility is good. |
| Intentionality | **deliberate-stale** — deliberate because custom wrappers exist, but likely worth revisiting as ecosystem SARIF support expands. |
| Recommendation | **investigate** |

### Finding D

| Field | Value |
|---|---|
| Subsystem | Rendering engine and dependency policy |
| Category | Wrong Abstraction Level |
| Current approach | Thin in-repo renderers over pattern data; explicit minimal dependency policy (Lip Gloss + x/term). |
| Alternative | Adopt larger TUI frameworks (e.g., Bubble Tea) for layout/state or external dashboard services. |
| Impact | **Low (negative if changed now)** — for this CLI’s scope, heavier frameworks add dependency and operational overhead without clear fitness gain. |
| Intentionality | **deliberate-valid** — dependency minimization is explicitly documented and still aligned with product scope. |
| Recommendation | **keep** |

### Finding E

| Field | Value |
|---|---|
| Subsystem | SARIF mapping pass structure |
| Category | Wrong Algorithm |
| Current approach | Multiple passes over SARIF data (`ComputeStats`, `TopFiles`, `GroupByFile`, per-group sorting). |
| Alternative | Single-pass aggregation that computes stats, top-file counters, and grouped rows in one traversal. |
| Impact | **Low** — asymptotics are already acceptable for expected SARIF sizes; current readability likely outweighs micro-optimization. |
| Intentionality | **deliberate-valid** (inference) — structure appears optimized for clarity/composability rather than raw throughput. |
| Recommendation | **keep** |

## 3) Overall classification summary

- **Deliberate and still valid**: Findings D, E.
- **Deliberate but stale**: Findings A, C.
- **Accidental**: Finding B.

## 4) Suggested next checkpoints

1. Prototype streaming report parse + streaming `go test -json` non-TTY path; measure peak RSS and time-to-first-summary.
2. Define ingestion strategy ADR: “custom multi-format forever” vs “SARIF-first normalization boundary”.
3. Keep renderer dependency posture unchanged unless product scope expands into interactive TUI workflows.

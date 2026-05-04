# fo

Focused build output renderer. Accepts SARIF and go test -json on stdin, renders for terminals, LLMs, or automation.

Language: Go 1.24+
Workspace: /Users/vcto/Projects/fo

## Architecture

```
stdin
  │
  ├─[1] read           internal/boundread (batch 256 MiB cap) | internal/lineread (stream)
  │
  ├─[2] sniff          cmd/fo/main.go: sniffSARIF, sniffGoTestJSON, report.HasDelimiter
  │
  ├─[3] parse          pkg/sarif (ReadBytes → ToReportWithMeta)
  │                    pkg/testjson (ParseBytes / Stream → ToReport)
  │                    pkg/report.ParseSections (multiplex --- tool: --- protocol)
  │
  ├─[4] Report (IR)    pkg/report/report.go — Findings, Tests, Diff, Notices
  │
  ├─[5] diff classify  pkg/state (sidecar .fo/last-run.json) + pkg/fingerprint + pkg/score
  │                    → attaches report.DiffSummary
  │
  ├─[6] mode pick      cmd/fo/main.go: resolveFormat (auto = TTY?human:llm)
  │
  ├─[7] render         pkg/view (human | llm | json)  → pkg/paint (bars, tables, sparklines)
  │                                                    → pkg/theme (color | mono)
  │
  └─[8] exit code      cmd/fo/main.go: exitCodeReport (0 clean | 1 findings/fail | 2 error)
                                                                                       │
                                                                                       ▼
                                                                                     stdout
```

Subcommands (cmd/fo/main.go): `fo wrap <name>` dispatches to pkg/wrapper/wrap{archlint,archlinttext,cover,diag,gobench,jscpd,leaderboard}; `fo wrap list`; `fo state reset`; `fo --version`; `fo --print-schema` (pkg/report.Schema).

Inputs: SARIF 2.1.0, go test -json, multiplex-delimited combo, hygiene formats (`# fo:status`, `# fo:metrics`, `# fo:tally`). Outputs: human (TTY), llm (piped), json.

## Package Structure

| Path | Role |
|---|---|
| `cmd/fo/` | CLI entry, flag parsing, format sniffing, subcommand dispatch |
| `pkg/report/` | Canonical `Report` struct + multiplex delimiter protocol |
| `pkg/sarif/` | SARIF 2.1.0 types, reader, builder, aggregates → Report |
| `pkg/testjson/` | `go test -json` stream parser → Report |
| `pkg/view/` | Renderers: human, llm, json; mode dispatch |
| `pkg/paint/` | Tufte-Swiss primitives: bars, sparklines, tables |
| `pkg/theme/` | v2 theme system (color/mono) |
| `pkg/state/` | Sidecar `.fo/last-run.json` for diff classification |
| `pkg/score/` | Severity scoring |
| `pkg/fingerprint/` | Finding identity for diff classification |
| `pkg/status/` | Hygiene format: PASS/FAIL/WARN/SKIP labeled rows |
| `pkg/metrics/` | Hygiene format: keyed numeric values (coverage, LOC, bench) |
| `pkg/tally/` | Hygiene format: count→label distributions (Leaderboard view) |
| `pkg/wrapper/wraparchlint/` | go-arch-lint JSON → SARIF |
| `pkg/wrapper/wraparchlinttext/` | go-arch-lint plain-text → SARIF |
| `pkg/wrapper/wrapcover/` | `go tool cover -func` → fo:metrics |
| `pkg/wrapper/wrapdiag/` | Line diagnostics (`file:line:col: msg`) → SARIF |
| `pkg/wrapper/wrapgobench/` | `go test -bench` → fo:metrics |
| `pkg/wrapper/wrapjscpd/` | jscpd JSON → SARIF |
| `pkg/wrapper/wrapleaderboard/` | plain `count label` → fo:tally |
| `internal/boundread/` | Bounded stdin reader (256 MiB cap) |
| `internal/lineread/` | Line-by-line reader for streaming mode |

## Key Design Decisions

- Report is the IR. Parsers produce it; renderers consume it.
- TTY auto-detection: `--format auto` → TTY=human, piped=llm.
- Exit codes: 0=clean, 1=findings/failures, 2=fo error.
- Deps: lipgloss + x/term only.
- Wrappers: each is a package exposing `Convert(in, out) error`. Dispatched by `switch` in `cmd/fo/main.go` (no interface, no registry).
- Adding a wrapper: new package under `pkg/wrapper/`, expose `Convert`, add a case to the wrap dispatch + import in `cmd/fo/main.go`.

## Search Scope
Skip: vendor, node_modules, build, .trash, dist, .git, .worktrees

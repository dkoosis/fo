# fo

Focused build output renderer. Accepts SARIF and go test -json on stdin, renders for terminals, LLMs, or automation.

Language: Go 1.24+
Workspace: /Users/vcto/Projects/fo

## Architecture

```
stdin → sniff (cmd/fo/main.go: sniffSARIF/sniffGoTestJSON)
      → parse  (pkg/sarif | pkg/testjson)
      → Report (pkg/report)
      → render (pkg/view → pkg/paint + pkg/theme)
      → stdout
```

Inputs: SARIF 2.1.0, go test -json. Outputs: human (TTY), llm (piped), json.

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
| `pkg/wrapper/wraparchlint/` | go-arch-lint JSON → SARIF |
| `pkg/wrapper/wrapdiag/` | Line diagnostics (`file:line:col: msg`) → SARIF |
| `pkg/wrapper/wrapjscpd/` | jscpd JSON → SARIF |
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

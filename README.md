# fo

Focused build output renderer. Reads SARIF, `go test -json`, or lightweight hygiene formats on stdin and renders for terminals, LLMs, or automation.

```
golangci-lint run --output.sarif.path=stdout ./... | fo
go test -json ./...                                 | fo
go vet ./... 2>&1                                   | fo wrap diag --tool govet | fo
```

## What it does

`fo` is a stdin-driven presentation layer. It auto-detects the input format, parses it into a canonical `Report` (the IR), classifies findings against a sidecar baseline (new / persistent / fixed), and renders one of three outputs:

- **human** — Tufte-Swiss styled terminal output (default on TTY)
- **llm** — token-dense plain text, no ANSI (default when piped)
- **json** — machine-parseable Report

Everything funnels through the same Report struct, so wrappers, hygiene formats, and native parsers all benefit from the same diff classification, scoring, and renderers.

## Install

```bash
go install github.com/dkoosis/fo/cmd/fo@latest
```

Requires Go 1.24+. Runtime deps: `lipgloss`, `golang.org/x/term`.

## Input formats

Auto-detected from stdin:

| Format | Trigger | Source |
|---|---|---|
| SARIF 2.1.0 | JSON shape | golangci-lint, gosec, staticcheck, custom tools |
| `go test -json` | JSON-line shape | `go test -json ./...` |
| tally | `# fo:tally` header (or bare `<count> <label>` rows) | hygiene scripts, `uniq -c` pipelines |
| status | `# fo:status` header | doctor scripts, contract checks |
| metrics | `# fo:metrics` header | coverage, benchmarks, sizes |

If stdin lacks a header, force a kind with `--as tally|status|metrics|diag`.

See [docs/guides/hygiene-formats.md](docs/guides/hygiene-formats.md) for the hygiene format reference, migration recipes, and `FO_STATE_DIR` notes.

## Wrappers

`fo wrap <name>` adapters convert third-party tool output into SARIF or a hygiene format, then pipe to `fo` for rendering:

```
archlint        go-arch-lint JSON → SARIF
archlint-text   go-arch-lint plain-text → SARIF
cover           go tool cover -func → fo:metrics
diag            file:line:col: msg → SARIF
gobench         go test -bench → fo:metrics
jscpd           jscpd JSON → SARIF
leaderboard     "<count> <label>" tally → fo:tally
```

`fo wrap list` (or `fo wrap list --json`) prints the current set.

## CLI

```
USAGE
  <input-command> | fo [FLAGS]
  <tool-output>   | fo wrap <name> [FLAGS]

FLAGS
  --format <mode>      auto | human | llm | json   (default: auto)
  --theme <name>       color | mono                (default: auto — color on TTY)
  --state-file <path>  Sidecar state file          (default: .fo/last-run.json)
  --no-state           Skip diff classification + sidecar I/O
  --state-strict       Exit non-zero (2) if sidecar save fails
  --stream             Stream go test -json incrementally (bypasses 256 MiB cap)
  --as <kind>          Format hint when stdin lacks a fo header

SUBCOMMANDS
  fo wrap <name>       Convert tool output to SARIF / hygiene format
  fo wrap list         List available wrappers
  fo state reset       Clear the diff baseline
  fo --version         Print build version
  fo --print-schema    Emit JSON Schema for the Report struct
```

## Exit codes

```
0   Clean — no errors or test failures
1   Failures — lint errors or test failures present
2   Usage error — bad flags, unrecognized input, stdin problems
```

Use the exit code, not stdout parsing, to gate CI steps.

## Diff classification

`fo` writes a sidecar at `.fo/last-run.json` after each run (override with `--state-file` or `FO_STATE_DIR`, disable with `--no-state`). The next run classifies each finding as **new**, **persistent**, or **fixed** by stable fingerprint, and surfaces those deltas in the rendered output.

`fo state reset` clears the baseline.

## Architecture

```
stdin
  ├─[1] read       (bounded 256 MiB / streaming)
  ├─[2] sniff      (SARIF / go test -json / hygiene / multiplex)
  ├─[3] parse      → Report (IR)
  ├─[4] diff       (vs sidecar baseline)
  ├─[5] mode pick  (TTY → human, piped → llm)
  ├─[6] render     (paint primitives + theme)
  └─[7] exit code  (0 / 1 / 2)
```

| Path | Role |
|---|---|
| `cmd/fo/` | CLI entry, flag parsing, format sniffing, wrap dispatch |
| `pkg/report/` | Canonical Report + multiplex delimiter protocol |
| `pkg/sarif/` | SARIF 2.1.0 reader/builder → Report |
| `pkg/testjson/` | `go test -json` parser → Report |
| `pkg/view/` | human / llm / json renderers |
| `pkg/paint/` | Tufte-Swiss primitives (bars, sparklines, tables) |
| `pkg/theme/` | Color / mono theme system |
| `pkg/state/` | Sidecar baseline for diff classification |
| `pkg/score/`, `pkg/fingerprint/` | Severity scoring + finding identity |
| `pkg/status/`, `pkg/metrics/`, `pkg/tally/` | Hygiene format parsers |
| `pkg/wrapper/wrap*/` | Tool-specific adapters |
| `internal/boundread/`, `internal/lineread/` | Stdin readers |

## Adding a wrapper

1. Create a package under `pkg/wrapper/` exposing `Convert(in io.Reader, out io.Writer) error`.
2. Add a case to the `wrap` dispatch in `cmd/fo/main.go` and import the package.
3. Wire a one-line description into the `wrap list` table.

No interface, no registry — a deliberate `switch`.

## License

MIT

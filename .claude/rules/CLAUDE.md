# fo

Focused build output renderer. Accepts SARIF and go test -json on stdin, renders for terminals, LLMs, or automation.

Language: Go 1.24+
Workspace: /Users/vcto/Projects/fo

‡ Go symbol questions → `snipe` (def/refs/callers/pack/impact/tests) before rg/Grep. rg = non-symbol text only.

## Architecture

```
stdin
  │
  ├─[1] read           internal/boundread (batch 256 MiB cap) | internal/lineread (stream)
  │
  ├─[2] sniff          cmd/fo/main.go: sniffSARIF, sniffGoTestJSON, multiplex.HasDelimiter
  │
  ├─[3] parse          pkg/sarif (ReadBytes → ToReportWithMeta)
  │                    pkg/testjson (ParseBytes / Stream → ToReport)
  │                    pkg/multiplex.ParseSections (--- tool: --- protocol)
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

Subcommands (cmd/fo/main.go): `fo wrap <name>` dispatches to pkg/wrapper/wrap{archlint,archlinttext,cover,coverprofile,diag,gobench,jscpd,leaderboard}; `fo wrap list`; `fo state reset`; `fo explain <id>` (resolve F-/T- handle from last run); `fo trend <rule-id>` / `fo replay [--since]` (run-log history); `fo --version`; `fo --print-schema` (pkg/report.Schema).

Inputs: SARIF 2.1.0, go test -json, multiplex-delimited combo, hygiene formats (`# fo:status`, `# fo:metrics`, `# fo:tally`). Outputs: human (TTY), llm (piped), json, github (Actions annotations, scoped to new findings via diff).

Addressable surface (fo-u15): every finding/test carries a short handle (`F-7a2`/`T-3f1`) = shortest unique fingerprint prefix, assigned by `report.AssignShortIDs[Stable]` after suppress/diff, pinned cross-run via `.fo/findings.json` snapshot. `fo explain` resolves it; `.fo/run-log.json` feeds trend/replay.

## Package Structure

| Path | Role |
|---|---|
| `cmd/fo/` | CLI entry, flag parsing, format sniffing, subcommand dispatch |
| `pkg/report/` | Canonical `Report` struct (pure IR) + JSON Schema |
| `pkg/multiplex/` | Multi-tool delimiter protocol (`--- tool: --- `): sniff + ParseSections |
| `pkg/sarif/` | SARIF 2.1.0 types, reader, builder, aggregates → Report |
| `pkg/testjson/` | `go test -json` stream parser → Report |
| `pkg/view/` | Renderers: human, llm, json; mode dispatch |
| `pkg/paint/` | Tufte-Swiss primitives: bars, sparklines, tables |
| `pkg/theme/` | v2 theme system (color/mono) |
| `pkg/state/` | Sidecars: `.fo/last-run.json` (diff), `.fo/findings.json` (ID snapshot+explain), `.fo/run-log.json` (trend/replay) |
| `pkg/score/` | Severity scoring |
| `pkg/fingerprint/` | Finding identity for diff classification |
| `pkg/status/` | Hygiene format: PASS/FAIL/WARN/SKIP labeled rows |
| `pkg/metrics/` | Hygiene format: keyed numeric values (coverage, LOC, bench) |
| `pkg/tally/` | Hygiene format: count→label distributions (Leaderboard view) |
| `pkg/scene/` | Cast-rail/narration: `Frame` + scene rendering (imported by view) |
| `pkg/cluster/` | Finding clustering: anchors, frames, normalization, IDs |
| `pkg/suppress/` | Finding suppression: match rules against findings |
| `pkg/wrapper/wraparchlint/` | go-arch-lint JSON → SARIF |
| `pkg/wrapper/wraparchlinttext/` | go-arch-lint plain-text → SARIF |
| `pkg/wrapper/wrapcover/` | `go tool cover -func` → fo:metrics |
| `pkg/wrapper/wrapcoverprofile/` | `-coverprofile` file → SARIF (note per uncovered block) |
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
- Deps: lipgloss + x/term + fsnotify (watch only).
- Wrappers: each is a package exposing `Convert(in, out) error`. Dispatched by `switch` in `cmd/fo/main.go` (no interface, no registry).
- Adding a wrapper: new package under `pkg/wrapper/`, expose `Convert`, add a case to the wrap dispatch + import in `cmd/fo/main.go`.

## Dev Workflow

‡ **Batch small fixes → one PR.** Several small/independent fixes in flight → ONE branch, ONE PR, one commit per fix. Full `check`/CI fires once at the PR (+ once on merge to main), NOT once per fix — a PR-per-one-liner serializes the queue behind build time. Each fix stays its own commit (traceable); PR body lists them. Review reads per-commit. Bundle by session/theme; ✗ mix a risky change in with trivial ones (it drags the whole PR's review bar up). **Default: auto-batch** — ≥2 small fixes queued → roll them onto one PR without asking.
‡ **PR ↔ beads.** Every PR body carries a `Closes:` trailer naming the beads it lands: `Closes: fo-abc, fo-def` (no bead → `Closes: none`). Squash-merge keeps the trailer in main's commit. On merge, close them with the landing ref: `bd close <ids> --reason "merged #<PR> (<sha>)"`. ✗ merge-then-forget — a bead whose code landed but stays open is a leak.

## Search Scope
Skip: vendor, node_modules, build, .trash, dist, .git, .worktrees

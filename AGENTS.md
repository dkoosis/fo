# fo — Agent Instructions

Focused build output renderer. Accepts SARIF and go test -json on stdin, renders for terminals, LLMs, or automation.

## Environment Setup

Setup is handled by `.codex/setup.sh` (auto-discovered by Codex on container creation).
Fallback: `source .codex/activate.sh` (auto-detects platform, links prebuilt binaries from `.bin/linux-{amd64,arm64}/`).

### Required tools

| Tool | Purpose | Example |
|------|---------|---------|
| `snipe` | Go symbol navigation (AST-indexed) | `snipe def Render`, `snipe callers ParseSARIF` |
| `golangci-lint` | Go linting (v2) | `golangci-lint run --output.text.path=stdout ./...` |
| `gofumpt` | Strict Go formatting | `gofumpt -w file.go` |
| `goimports` | Fix imports | `goimports -w file.go` |
| `jq` | JSON processing | `jq '.runs' sarif.json` |

### Optional tools

| Tool | Purpose | Example |
|------|---------|---------|
| `govulncheck` | Vulnerability scanning (not in CI) | `govulncheck ./...` |

### Orientation workflow
```bash
snipe def <Symbol>            # jump to any definition
snipe callers <Symbol>        # find who calls a function
snipe search "pattern"        # text search
make help                     # show available targets
make qa                       # full QA pass (build + test + lint)
```

## Project Structure

```
cmd/fo/                    CLI entry, flags, subcommands (fo, fo wrap sarif)
pkg/pattern/               Pure data structs: Summary, Leaderboard, TestTable
pkg/sarif/                 SARIF types, reader, stats, builder
pkg/testjson/              go test -json stream parser
pkg/mapper/                SARIF → patterns, testjson → patterns
pkg/render/                Renderer interface + human, llm, json implementations + themes
pkg/stream/                Streaming go test -json renderer (TTY live output)
pkg/wrapper/               Wrapper plugin interface, registry
pkg/wrapper/wrapdiag/      Line diagnostics → SARIF
pkg/wrapper/wrapjscpd/     jscpd JSON → SARIF
pkg/wrapper/wraparchlint/  go-arch-lint JSON → SARIF
internal/detect/           Format sniffing (SARIF vs go test -json)
internal/report/           Report delimiter protocol (multi-tool pipelines)
```

## Key Rules

- Patterns are pure data, not renderers (renderer decides presentation)
- golangci-lint v2: use `--output.text.path=stdout`, not `--out-format`
- Exit codes: 0=clean, 1=failures, 2=fo error
- Dependencies: lipgloss + x/term only
- Build system: Makefile (not mage)

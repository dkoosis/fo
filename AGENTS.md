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

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->

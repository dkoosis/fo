# fo

Focused build output renderer. Accepts SARIF and go test -json on stdin, renders for terminals, LLMs, or automation.

Language: Go 1.24+
Workspace: /Users/vcto/Projects/fo

## Architecture

```
stdin → detect format → parse → map to patterns → render → stdout
```

Two inputs: SARIF 2.1.0, go test -json
Three outputs: terminal (TTY), llm (piped), json (--format json)

## Package Structure

- `cmd/fo/` — CLI entry, flags, subcommands (fo, fo wrap sarif)
- `pkg/pattern/` — Pure data structs: Summary, Leaderboard, TestTable, Sparkline, Comparison
- `pkg/sarif/` — SARIF types, reader, stats, builder
- `pkg/testjson/` — go test -json stream parser
- `pkg/mapper/` — SARIF → patterns, testjson → patterns
- `pkg/render/` — Renderer interface + terminal, llm, json implementations + themes
- `internal/detect/` — Format sniffing (SARIF vs go test -json)

## Key Design Decisions

- Patterns are pure data, not renderers (renderer decides presentation)
- TTY auto-detection: `--format auto` (default) → TTY=terminal, piped=LLM
- Exit codes: 0=clean, 1=failures, 2=fo error
- Dependencies: lipgloss + x/term only

## Session Start
1. `set_workspace /Users/vcto/Projects/fo` — auto-loads `n:boot:fo`
2. `search_nugs({tags: ["project:fo"], limit: 10})` — project context

## Search Scope
Skip: vendor, node_modules, build, .trash, dist, .git, .worktrees

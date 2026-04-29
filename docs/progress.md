# fo — Progress

Read first every session.

## Current

Focused build output renderer. SARIF + go test -json on stdin, renders for terminals, LLMs, or automation.

### Last session

- Architecture review completed: Yellow score, clean 3-layer acyclic graph
- Closed PR #215 (Codex placeholder, zero value)
- Designed Makefile to replace magefile — user approved Makefile over mage

### Ready

Create Makefile with QA regimen — design approved, ready to implement.

1. Write `Makefile` with targets: `all` (default), `build`, `test`, `lint`, `fmt`, `clean`, `lint-sarif`
2. `rm -rf internal/version/` — dead package, zero references
3. `go build ./... && go test ./...` — confirm clean
4. Commit both changes together

Design notes: Makefile over magefile (user chose zero-dep). Targets call `go build`, `go test -race -cover`, `go vet`, `golangci-lint run` directly. No abstractions.

## Backlog

Create Makefile | remove internal/version | unit tests for pkg/render (674 LOC) | unit tests for pkg/mapper (372 LOC) | add .go-arch-lint.yml for layering enforcement | delete magefile.go.bak after Makefile works

## Decisions

- Makefile over magefile (zero-dep, user chose)
- Patterns are pure data, not renderers
- TTY auto-detection: `--format auto` (default) → TTY=terminal, piped=LLM
- Exit codes: 0=clean, 1=failures, 2=fo error
- Dependencies: lipgloss + x/term only

## Traps

- Pre-commit hook blocks .md files outside allowed locations — boot.md stays untracked
- golangci-lint v2.x: use `--output.text.path=stdout`, not `--out-format`
- `internal/magetasks/` and `fo` console lib are deleted — magefile.go.bak won't compile as-is, reference only

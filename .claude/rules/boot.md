# Boot
updated: 2026-05-03

→ epic fo-kxq (hygiene formats) underway. fo-kxq.1 (status parser) ✓. Next: fo-kxq.2 (status view renderer).

state: φ docs/superpowers/plans/2026-05-03-hygiene-formats.md

✓ done
- fo-kxq epic + 13 children filed, deps wired
- fo-kxq.1 status TSV parser shipped (pkg/status, 4 tests green)

‡ traps
- status format is TSV-after-state w/ space-only fallback — don't revert to strings.Fields
- Task 10 is `gobench` (raw `go test -bench`), NOT benchstat tabular — that's deferred
- gh-issue meta: epic only, children inherit

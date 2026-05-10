# Boot
updated: 2026-05-09

→ pick from `bd ready`. Watch chain next: fo-u15.1.4 (TUI rerender loop + signal handling).

state: untracked `docs/feedback/` — pre-existing, triage when convenient.

✓ done
- fo-u15.1.3 shipped (4bef86a): test-outcome delta tracking (NewFailures/FixedFailures/FlakyTests) in DiffSummary; audit clean.
- fo-u15.1.2 shipped (ded9ee3): fsnotify watcher + 250ms debounce; `-source=fs|stdin` flag.
- Drive-by: race in TestWatchLoop_ExitsOnCtxCancel fixed (atomic.Int64).

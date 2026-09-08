# fo roadmap

★ Turn the messy output of build/test/lint tools into dense, legible signal — for
a human at a TTY, an LLM in a pipe, or a machine consuming JSON.

fo is a streaming presentation filter: stdin → IR (`Report`) → render. No TUI, no
event loop, no interactive state. Full statement: `.claude/rules/north-star.md`.

## Epics

Ordered, one line per epic. Progress is never written here — it derives at read
time from the bd DAG joined against these ids.

_No open epics tracked in bd as of this writing — current work is filed as
standalone tasks/bugs off `bd ready`._

## Non-goals

- Interactive TUI (Bubble Tea, full-screen apps)
- Long-lived daemon, server, or watcher-as-service (watch mode is a one-shot loop)
- Owning tool invocation — fo reads stdin; callers run the tools
- Format-specific renderers — everything goes through `Report`

## Resources

- `.claude/rules/north-star.md` — full north star + design contract + triage yardsticks
- `.claude/rules/CLAUDE.md` — architecture, package structure, dev workflow
- `.beads/` — work queue (epics, tasks, dependencies)

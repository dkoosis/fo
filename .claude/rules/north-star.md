# fo north star

The yardstick for triage: does this work move fo toward what it's for, or away?

## What fo is

A streaming presentation filter. stdin → IR (`Report`) → render. No TUI, no event loop,
no interactive state. ANSI cursor control for in-place updates is the ceiling.

## What fo is for

Turning the messy output of build/test/lint tools into dense, legible signal —
optimized for the reader (human at a TTY, LLM in a pipe, or machine consuming JSON).
The reader's attention is the scarce resource; fo defends it.

## Load-bearing decisions

1. **Report is the IR.** Every parser produces it; every renderer consumes it. New
   inputs become parsers; new outputs become renderers. ✗ side channels.
2. **Auto-detect by default.** TTY → human, piped → llm. `--format` only when forced.
3. **Exit codes are the contract.** 0 clean | 1 findings/failures | 2 fo error.
   Callers parse exit code, not output.
4. **Wrappers are thin.** `pkg/wrapper/*` adapters convert foreign formats to SARIF
   or hygiene formats, then exit. They don't render.
5. **Diff classification is built in.** Sidecar state (`.fo/last-run.json`) →
   new/persistent/fixed. The reader sees what changed, not just what is.
6. **Shape-aware rendering.** Data shape determines visual idiom — counts → leaderboard,
   ratios → progress bar, time series → sparkline, key/value → metrics table, pass/fail
   rows → status grid. Hygiene formats (`fo:tally`, `fo:status`, `fo:metrics`) are
   *shape declarations*, not just data carriers. Aspiration: fo recognizes shape from
   the data itself when no declaration is present.

## Design contract

- **Tufte-Swiss.** Data-ink ratio, sparklines, small multiples, small effective
  differences, no chartjunk. See `docs/TUFTE_PRINCIPLES.md` for the long form.
- **Cognitive load.** Visual hierarchy makes errors/trends/anomalies pop against
  context. Density modes let the reader choose how much signal at once.
- **Two readers, one IR.** human and llm renderers are peers — neither is the
  "real" one. llm output is not a degraded human view; it's a different reader.

## Non-goals

- ✗ Interactive TUI (Bubble Tea, full-screen apps)
- ✗ Long-lived daemon, server, or watcher-as-service (watch mode is a one-shot loop)
- ✗ Owning tool invocation — fo reads stdin; callers run the tools
- ✗ Format-specific renderers — everything goes through `Report`

## Triage yardsticks

When deciding whether a bead/issue is worth executing, ask:

| Question | Keep if … |
|---|---|
| Does it preserve IR purity? | Yes — or makes it purer |
| Does it improve reader experience (human, LLM, or JSON)? | Yes |
| Does it tighten the contract (exit codes, auto-detect, wrappers)? | Yes |
| Does it apply Tufte/cognitive-load principles to a render? | Yes |
| Does it teach fo a new shape, or improve shape recognition? | Yes |
| Does it close a real correctness gap (bug, race, leak, parse error)? | Yes |
| Does it add a feature outside the streaming-filter model? | ✗ close/defer |
| Does it duplicate a renderer or bypass the IR? | ✗ close/defer |

If a bead is ambiguous against this doc, surface it — don't guess.

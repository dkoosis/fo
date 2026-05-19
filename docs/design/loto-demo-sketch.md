# loto demo sketch — `whoami.txtar` end-to-end

**Date:** 2026-05-18
**Purpose:** Make Open Q #2 (frame model) and the cast-rail story concrete by hand-tracing the smallest loto scenario through fo. Sketch only; no implementation.

## Source: the testscript

`cmd/loto/testdata/script/whoami.txtar` (10 lines) — the simplest .txtar in the loto suite:

```
# Identity round-trip: pinned LOTO_AGENT_ID surfaces the right handle.

env LOTO_AGENT_ID=$ALICE
loto whoami
stdout 'AliceTester'

env LOTO_AGENT_ID=$BOB
loto whoami
stdout 'BobTester'
! stdout 'AliceTester'
```

Two actors (Alice, Bob), one command each, identity assertion. Picked because it has narration potential, two actors, and zero failure paths — the smallest non-trivial scene.

## Step 1 — Demo runner produces scene transcript on stdout

A `make demo` runner (NOT YET BUILT) walks the .txtar, executes each `loto …` step against the real binary, and emits `# fo:scene` to stdout. For `whoami.txtar` the runner would emit:

```
# fo:scene title="loto: every session has a handle" actors="AliceTester,BobTester"

## 01 · two agents land in the same repo

> a fresh session needs to know what it's called.

> loto pins a handle from $LOTO_AGENT_ID — same env var, two different agents, two different identities.

@AliceTester $ loto whoami
  AliceTester

> Bob opens his own session, same repo, different env.

@BobTester $ loto whoami
  BobTester

> identity is stable per-session: Alice never appears as Bob.
```

Notes on this transcript:

- The header (`# fo:scene …`) is the hygiene-format declaration `pkg/scene` already recognizes (scene.go:43 `HeaderPrefix`).
- Narration (`> …`) is editorial — the runner needs a per-step narration map or inline `# narrate: …` comments in the .txtar. **This is the first design choice the runner forces.** Probably the .txtar grows annotation comments.
- Output (`  AliceTester`) is 2-space-indented per scene.go grammar.
- Assertion lines (`stdout 'AliceTester'`) are NOT in the transcript — the runner consumes them as test gates, not narration. A failing assertion would surface as `(exit N)` on the command beat.

## Step 2 — Human render (what `fo` prints to a TTY)

Piping the transcript above into `fo` (auto-detects scene, picks human renderer because TTY). Based on `pkg/view/scene_human.go:44-107` the layout is: optional title, then per-act `─────…` rule + `N · title` heading + narration (dimmed, 2-space indent) + command beats (colored actor + `❯` + cmd, then 2-space output).

```
loto: every session has a handle

────────────────────────────────────────────────────────────
01 · two agents land in the same repo

  a fresh session needs to know what it's called.
  loto pins a handle from $LOTO_AGENT_ID — same env var, two
  different agents, two different identities.
AliceTester ❯ loto whoami
  AliceTester
  Bob opens his own session, same repo, different env.
BobTester ❯ loto whoami
  BobTester
  identity is stable per-session: Alice never appears as Bob.
```

(In a real TTY: title is bold; rule + narration are muted/dim; `AliceTester` and `BobTester` get stable per-actor palette colors hashed via `actorStyle`; `❯` is bold. Mono terminal / `NO_COLOR=1` strips the colors but keeps the same structure.)

**Single rendered string. No interleaving of writes with logic outside the renderer.** This is the cast-rail attach point.

## Step 3 — Cast render (sketch, NOT IN SCOPE TONIGHT)

A cast rail would walk the same `scene.Scene`, but instead of one composite string it would emit one asciinema event per beat (or per act), choosing delays authorially:

```
{"version":2,"width":80,"height":24,"timestamp":...,"title":"loto: every session has a handle"}
[0.0, "o", "loto: every session has a handle\r\n\r\n"]
[0.5, "o", "──────…\r\n01 · two agents land in the same repo\r\n\r\n"]
[1.0, "o", "  a fresh session needs to know what it's called.\r\n"]
[2.5, "o", "  loto pins a handle…\r\n"]
[4.0, "o", "AliceTester ❯ loto whoami\r\n"]
[4.6, "o", "  AliceTester\r\n"]
…
```

Two things this sketch makes concrete:

1. **A "frame" for cast = a (delay, string-chunk) pair.** The string-chunk is whatever ONE beat renders to. So cast doesn't need a frame type distinct from "rendered beat text" — it needs *a renderer that emits per-beat strings instead of one composite string*. That's exactly the seam the audit (`docs/design/view-layer-audit.md`) identifies in `stream.go:RenderStreamMode`.

2. **Authorial delay table.** Narration gets a longer delay than command output. Pauses between acts. The delay policy is a separate concern from the renderer — probably a small struct `BeatTiming{ Narration, Command, OutputLine, ActSeparator time.Duration }` consumed by the cast emitter.

## What this sketch surfaces for the open questions

- **Q #2 (frame model):** A frame for cast is just a beat's rendered text. We don't need a new `Frame` type — we need scene_human refactored to expose `RenderBeat(beat) string`, then cast composes the cast file from those. See deliverable 4 for a concrete struct sketch.
- **Q #5 (state model):** `whoami` has no mutable world state (no locks, no shared resources). The next-smallest scenario (`lock_happy.txtar`) does — that's where the "state grid" question gets real. **Not answered tonight.**
- **Runner responsibility:** The runner OWNS narration injection and assertion handling. fo doesn't see assertions. This is a clean cut.

## Open question this sketch raises for dk

(Logged to `three-output-rails.md` Questions for dk if confirmed.) Should narration live in the .txtar itself (annotation comments → demo runner extracts) or in a sidecar map (`whoami.demo.yaml`)? The first keeps things colocated; the second keeps testscript pure.

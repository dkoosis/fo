# Three Output Rails: Vision & Open Questions

**Status:** draft / pre-design
**Date:** 2026-05-18
**Drivers:** dk, Trixi (initial brainstorm)

## The Frame

```
SARIF | go-test-json | scene | metrics | tally | status   ──┐
                                                            │
                                                          [ fo ]
                                                            │
              ┌─────────────────────┬─────────────────────┐
              ▼                     ▼                     ▼
        llm (agent)          human (static)         cast (playback)
        token-dense           ascii widgets         asciinema events
        no chrome             bar/leaderboard/      timed frames
        deterministic         table/sparkline       shareable URL
```

One ingest pipeline. One pure model layer. Three output rails. Each rail is a different *consumer* of the same internal representation.

## The Three Rails

### llm — agent-optimized
- **Audience:** Claude and other agents inside loops.
- **Shape:** token-dense, structured (key=value rows, deterministic sort, glyph severity), no ANSI.
- **Status today:** mostly built (`view.RenderSceneLLM`, llm-mode dispatch, claudish glyphs).
- **Constraints:** byte-identical output for identical input. No prose padding. First line summarizes triage counts.

### human — static ASCII presentation
- **Audience:** developers reading a one-shot terminal print.
- **Shape:** Tufte-Swiss layout. Widget set selected by the input (or `--view` override): bar, sparkline, leaderboard, table, summary, scene transcript.
- **NOT bubbletea.** Static render, no animation, no keyboard. Bubbletea would only earn its weight later for *interactive* drill-down (sort/filter); animation is handled by the cast rail.
- **Status today:** scaffolding exists (`pkg/paint`, `pkg/theme`, `pkg/view`), but underdeveloped vs. the llm rail. Widget picker isn't generalized.

### cast — animated playback
- **Audience:** humans watching a story unfold. Demos, README gifs, onboarding.
- **Shape:** asciinema cast file (`(timestamp, output)` event stream). Plays in any terminal via `asciinema play`, in a browser via asciinema-player, scrubable, shareable as a URL.
- **Generation is programmatic, not screen-recorded.** fo emits the cast directly: walk the parsed model, for each beat write a frame with a chosen delay.
- **Animation does not require realtime.** Delays are authorial choices.
- **Status today:** not built.

## Why This Shape

Cast is just **human rail + a clock**. Bubbletea is **human rail + a keyboard**. If the view layer returns pure models / string frames (rather than writing directly to `io.Writer`), all three rails come from the same code path.

The architectural bet: **invest the next round of effort in the human rail's view layer, designed from day one to feed all three rails.** Don't build stdout-only and refactor later.

## The Forcing Function: loto demo

Loto's testscript `.txtar` files (`cmd/loto/testdata/script/*.txtar`) are already narrated scenarios with multiple actors. `pkg/scene` was built explicitly for this — its doc names `make demo` as the intended consumer.

Concretely the loto demo wants:
1. Walk a `.txtar`, execute each `loto …` step against the real binary, capture stdout.
2. Emit a `# fo:scene` transcript on stdout.
3. Pipe into `fo` → either human (one-shot pretty render) or cast (animated playback).
4. README ships the cast URL; `make demo` runs it in-terminal.

Norton Disk Defragmenter is the design reference: plain-English narration above each step, a visible state grid (agents, files, locks) that *changes* across beats, the satisfaction of watching invisible operations become legible.

## Open Architectural Questions

1. **View layer purity.** Does `view.RenderXHuman` currently return strings/models, or write to `io.Writer`? If the latter, refactor before adding rails. (Needs audit — flagged but not yet checked.)
2. **Frame model.** What is a "frame"? For cast, it's clearly `(delay, screen-contents)`. For static human, it's the final composite. For bubbletea (future), it's `View()`. Can one type serve all three, or do we need an intermediate?
3. **Widget picker.** Who chooses bar vs. leaderboard vs. table — the input shape (auto), an explicit `--view`, or both? What's the override grammar?
4. **State diff in cast.** Does each cast frame redraw the full screen, or emit just the delta? Full redraw is simpler and most terminals handle it; deltas are smaller.
5. **State model for scenarios.** A loto scene has a *world state* (agents, locks) that mutates. Does `pkg/scene` own that state, or does the producer (loto) hand fo a pre-computed state per beat?
6. **Cast at what layer.** Does cast emission live in `pkg/view` (alongside human/llm renderers), in its own `pkg/cast`, or as a wrapper around the human renderer with a clock?

## Open Requirements Questions

1. **Cast playback target.** Terminal-only via `asciinema play`, or also embeddable in web docs? (Affects whether we need asciinema v2 format strictly.)
2. **Recording loto's real CLI.** Is the demo (a) a *script* fo plays back, or (b) a *recording* of a real test run? If (b), the scenarios become regression-detectable. If (a), they're polished marketing.
3. **Widget vocabulary.** What's the v1 widget set? Proposed: scene transcript, leaderboard, table, bar, sparkline, summary. Anything else MVP?
4. **Theme parity.** Does cast respect NO_COLOR / theme like human does, or always ship the color version?
5. **Bubbletea later, or never?** If interactivity is wanted, where does it live — in `fo` itself, or a separate `fo-tui` binary?
6. **Distribution.** Demo casts checked into the repo (binary artifacts), regenerated by `make demo`, or hosted externally (asciinema.org / self-host)?

## Suggested Next Session

1. **Audit `pkg/view` purity.** Cheap, high-leverage. Determines whether the three-rails design is a refactor or a clean extension.
2. **Sketch the loto demo end-to-end** with one concrete `.txtar` → scene transcript → human render → cast file. Smallest end-to-end loop. Surfaces the real frame-model and state-diff questions.
3. **Decide widget picker grammar** before generalizing it. `--view` flag? `# fo:view leaderboard` header? Auto-detect? Pick one.

## Questions for dk

1. **Narration source for loto demo.** Should per-step narration live as annotation comments inside the `.txtar` (colocated, but pollutes testscript) or as a sidecar map like `whoami.demo.yaml` (clean separation, but two files to keep in sync)? Surfaced by the `whoami.txtar` sketch — `docs/design/loto-demo-sketch.md`. Trixi leans sidecar but doesn't know loto's conventions well enough to call it.

## Cross-References

- `pkg/scene/scene.go` — scene format parser, names `make demo` explicitly
- `pkg/view/scene_human.go`, `pkg/view/scene_llm.go` — current rail renderers
- `pkg/paint/paint.go` — pure visual primitives (bar, sparkline)
- `pkg/theme/theme.go` — mono + color presets, NO_COLOR handling
- `docs/design/philosophy.md` — fo's "why"
- `docs/design/architecture.md` — fo's "how today"
- loto: `cmd/loto/testdata/script/*.txtar` — the forcing-function scenarios
- loto: `cmd/loto/script_test.go` — testscript wiring; demo runner would mirror this

# Cast Rail & the Scene-vs-Report Decision

**Status:** decided
**Date:** 2026-06-14
**Supersedes open question:** three-output-rails.md §"Open Architectural Questions" #2 (Frame model), #6 (cast at what layer)
**Amends:** north-star.md Decision #1

## The question

`pkg/scene` is a second pipeline that bypasses `Report`. `cmd/fo/main.go:319`:

```
if scene.IsHeader(input) → renderScene → scene.Parse → view.RenderSceneLLM/Human
```

It never constructs a `Report`. So fo has two IRs: `Report` (a *snapshot* — findings,
tests, metrics) and `Scene` (a *time-ordered narrative* — acts → beats → commands with
narration and per-actor exit codes).

This collides two prior calls:

- **North-star Decision #1 (as written):** "Report is the IR … ✗ side channels." Scene
  is a side channel.
- **Locked call (boot.md):** "Frame{} in pkg/scene" — treats Scene as a legitimate peer.

## Decision

1. **Report stays the singular IR for tool-output snapshots.** Non-negotiable. This is
   fo's daily driver (CI, dev loops). The biggest long-term threat is the IR
   fragmenting — every new format inventing its own pipeline. Decision #1's purity
   prevents that and must hold hard. New **formats** are always parsers to Report.

2. **Scene is a bounded exception, not a peer or a precedent.** Forcing a multi-actor
   temporal narrative into Report would corrupt Report's flat snapshot shape — also bad
   long-term. Scene earns a separate IR because it's a genuinely different *shape*, not
   a new format. Decision #1 is amended to separate **format** (→ always a parser to
   Report) from **shape** (→ own IR only by exception, high bar). The exception list is
   exactly: snapshot = Report, narrative = Scene.

3. **`Frame{}` lives in `pkg/scene` — do not promote it to a universal rail interface.**
   The cast rail is intrinsically a Scene concept and feeds from Scene only.

## Why Frame is not universal

The "Frame unifies Report and Scene so the rails stay single" reconciliation was
considered and rejected. Pressure-test what each rail consumes:

| Rail | Consumes | Report feeds it? | Scene feeds it? |
|---|---|---|---|
| llm | token-dense snapshot | ✓ | ✓ (flattened transcript) |
| human | Tufte widget snapshot | ✓ | ✓ (scene transcript widget) |
| cast | timed frames | **✗ degenerate** | ✓ (one beat = one frame) |

An animated `Report` is meaningless — a SARIF snapshot has no time axis; cast → Report
is one frame or fake animation. So **cast is Scene-native**, and `Frame` is a
Scene-native type, not a cross-IR abstraction. Report feeds two rails; Scene feeds
three. Building a grand unified Frame layer is speculative generality for a need Report
doesn't have.

The view-layer audit (`view-layer-audit.md`) already shows the rails share enough: the
pure `Render → string` core. That's the shared machinery. Don't manufacture more.

## Consequences

- ✓ Everyday core (Report) stays clean and singular — extensible for years.
- ✓ Scene/cast (the demo/README rail) lives in a fenced lane that can't metastasize.
- ✓ No effort spent on a Frame abstraction whose unifying premise is false.
- Cost: one north-star amendment (format-vs-shape) + the standing "Scene is an
  exception, not a precedent" guard.

## What this unblocks

The cast rail can now be specced against `pkg/scene` directly:

- `Frame` = `(delay, screen-contents)`, owned by `pkg/scene`.
- Cast emission taps the per-beat render — each beat → one rendered string → one
  timestamped asciinema event. The `stream.go:RenderStreamMode` seam
  (`view-layer-audit.md` "Verdict") is the model: render to string, then wrap with a
  clock instead of writing to a sink.
- Cleanup before scene cast: `scene_human.go` / `scene_llm.go` need
  `RenderSceneXxxString(scene) string` companions (moderate, mechanical — see audit).

Remaining open questions (full-screen redraw vs delta, playback target, distribution)
stay in three-output-rails.md §"Open Requirements Questions" — they're cast-rail
implementation details, not IR-shape decisions, and don't block this call.

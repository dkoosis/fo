# pkg/view Purity Audit

**Date:** 2026-05-18
**Auditor:** Trixi (subagent sweep)
**Question answered:** Open Q #1 from `docs/design/three-output-rails.md` — does `pkg/view` return strings/models, or write to `io.Writer`?

## Why this matters

The cast rail needs to capture frames as discrete strings to emit timed asciinema events. If renderers write straight to a writer (and interleave logic with writes), cast can't intercept cleanly without buffering tricks. "Pure" (returns string) = three rails is a clean extension. "Impure throughout" = significant refactor required first.

## Per-file findings

| File | Exported API shape | Notes | Refactor cost |
|------|--------------------|-------|---------------|
| `bullet.go` | pure (returns `string` / `[][]string`) | `renderBullet`, `renderClusterBlock`, `renderGrouped` return strings; no io | trivial |
| `clean.go` | pure | `renderClean` returns string | trivial |
| `cluster.go` | pure | helpers + `NewExpandSet`; no writers | trivial |
| `delta.go` | pure | `renderDelta` returns string | trivial |
| `headline.go` | pure | `renderHeadline`, `renderAlert` return strings | trivial |
| `leaderboard.go` | pure | `renderLeaderboard` returns string | trivial |
| `metrics.go` | **impure** | `RenderMetricsLLM/Human(w io.Writer, ...)` — `fmt.Fprintf` to writer (L17, L38) | trivial (swap to `strings.Builder`) |
| `multiples.go` | pure | `renderSmallMultiples` returns string | trivial |
| `pickview.go` | pure | `PickView*` returns `ViewSpec` (IR → view-IR) | trivial |
| `render.go` | **pure — DISPATCH** | `Render(spec, t, width) string` (L21) — central type switch returning string | trivial (already there) |
| `scene_human.go` | **impure** | `RenderSceneHuman(w io.Writer, s scene.Scene) error` — many `Fprintf` calls threaded through helpers | moderate (multiple internal writers) |
| `scene_llm.go` | **impure** | `RenderSceneLLM(w io.Writer, ...)` — Fprintln/Fprintf throughout | moderate |
| `status.go` | **impure** | `RenderStatusLLM/Human(w io.Writer, ...)` — Fprintf to writer | trivial |
| `stream.go` | **impure** (legitimately) | `RenderReport*`, `RenderStream*` take `io.Writer`; L26 calls `Render(...) → string` then one `Fprintln` (L30). Stream variant loops snapshots. | trivial for batch; streaming inherently needs a sink |
| `view.go` | pure | Type defs + `isViewSpec()` marker methods only | trivial |

## Notes

The core view pipeline is **already pure**: `PickView → Render → string`. The `io.Writer`-shaped surface is a thin outer skin around the pure core.

- `stream.go` is the architectural seam. `stream.go:26` shows the pattern cast will reuse: call `Render(...)` to get a string, then a single `Fprintln` to the sink. Batch `RenderReport*` entries could be paralleled by `RenderReportString(...) string` for free.
- Only `RenderStreamMode` is genuinely sink-shaped (loops over a channel emitting frames as they arrive) — which is **exactly the seam cast wants to tap**. Each iteration produces one rendered string per snapshot; cast just timestamps and wraps instead of writing.
- `metrics.go` and `status.go` are trivial Fprintf-to-Builder swaps — mechanical.
- `scene_*.go` thread `w io.Writer` through several internal helpers (`renderAct`, `writeBeats`, `writeSceneHeader`). Refactoring them to return strings is moderate but mechanical — no streaming semantics, no interleaving with non-render logic. These are the scene transcript renderers — the loto demo will rely on them, so they're the highest-priority cleanup target.

## Verdict

**Three rails is a clean extension.**

The dispatch entry point (`render.go:Render`) already returns a string. `stream.go:RenderStreamMode` is the natural cast attach point — each per-snapshot `Render(...)` call IS a frame. No deep refactor needed.

Cleanup required for full coverage (all of it trivial or moderate, all mechanical):

1. `scene_human.go` / `scene_llm.go` — refactor `RenderSceneXxx(w, scene)` to also expose `RenderSceneXxxString(scene) string`. Moderate, but the threading is shallow.
2. `metrics.go` / `status.go` — swap writer-based bodies for `strings.Builder`, return string, keep thin `Fprint` wrappers for backward compat. Trivial.
3. `stream.go` — add `RenderReportString` companions next to `RenderReport*`. Trivial.

None of this is blocking. The cast rail could be prototyped today against the already-pure ViewSpec path; scene cast follows after step 1.

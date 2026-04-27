# fo-7f5.4 — Views layer design

**Status:** draft for dk review · 2026-04-26
**Bead:** fo-7f5.4 · **Depends on:** pkg/paint, pkg/theme, pkg/report (all landed)
**Blocks:** fo-7f5.5 (pickView), fo-7f5.7 (streaming), fo-40z.2 (delta)

## Scope

Build `pkg/view`: the layer that sits between `report.Report` and `pkg/paint`.
A `ViewSpec` is one of eight variants — each carries the data its variant
needs and renders to a string given a `theme.Theme`. No selection logic
here (that's pickView in 7f5.5); no parsing (that's substrate). Just
data → string.

Eight variants from the epic: Clean, Bullet, Grouped, Leaderboard,
Headline, Alert, Delta, SmallMultiples.

## Locked already (from epic + 4dv)

- pkg name: `pkg/view`
- one Theme value threaded through render
- no box-drawing; alignment via `paint.Columnize`
- Bullet/Grouped surface `FixCommand` as a copy-pastable line under the finding
- Section is a labeled block of any variant — no chrome
- mono is the base; color overlays severity/outcome only

## Three open decisions

### 1. Sum-type encoding

Go has no sum types. Three plausible shapes:

**A. Interface + concrete structs** (idiomatic, dispatch by method)
```go
type ViewSpec interface { Render(theme.Theme) string }
type Bullet struct { Items []BulletItem }
type Leaderboard struct { Rows []LbRow; Total float64 }
// ...
func (b Bullet) Render(t theme.Theme) string { ... }
```
✓ extensible, idiomatic, easy to test per-variant
✗ "sum" is by convention only — nothing prevents a third party adding a 9th
✗ pickView returns `ViewSpec`; callers can't switch exhaustively without type switch

**B. Tagged union struct** (one struct, kind + payloads)
```go
type Kind int
const (KindClean Kind = iota; KindBullet; ...)
type ViewSpec struct {
    Kind Kind
    Bullet *BulletData
    Leaderboard *LbData
    // ...
}
```
✓ exhaustive switch is natural; closed set enforced structurally
✗ ugly — most pointer fields nil at any time
✗ memory waste, awkward construction

**C. Closed-interface pattern** (interface with unexported marker method)
```go
type ViewSpec interface { isViewSpec() }
type Bullet struct { ... }
func (Bullet) isViewSpec() {}
// rendering lives in package-level func Render(ViewSpec, theme.Theme) string
// with a type switch — the switch IS the closed-set check
```
✓ closed set (only this package can satisfy `isViewSpec`)
✓ exhaustive `switch v := spec.(type)` in one place — easy to audit
✓ no method bloat on each variant
✗ rendering is a 200-line type switch instead of small per-variant files

**Recommendation: C.** The switch becomes the spec. Adding a 9th variant
forces touching one file, not nine. Per-variant complexity stays small
(each variant is ~20 lines of paint calls); centralizing render also
centralizes theme threading and column-budget math.

If we later regret it, A is a one-pass refactor.

### 2. Delta wrapping

Delta is "value + arrow + magnitude vs prior run." It's a *modifier* on
some other view, not a peer. The epic notes it as "composable wrapper
over another view."

**A. Delta as a variant** holding `Inner ViewSpec` + arrow data
```go
type Delta struct { Inner ViewSpec; Buckets []DeltaBucket }
```
Render: paint inner, then a footer line per bucket with arrow.

**B. Delta as a *field* on every variant** — each renders its own arrow
✗ leaks comparison logic into 7 places. Bad.

**C. Delta as a *post-processor* outside the spec** — renderer returns
string, then a second function appends the diff
✗ loses access to per-row context (which row went up, which down)

**Recommendation: A.** Inner-holding variant. Renderer's switch case for
Delta calls `Render(d.Inner, theme)` first, then paints the bucket
strip. The inner can be Bullet, Leaderboard, or SmallMultiples — the
three views where row-level deltas matter. For Headline/Alert/Clean,
pickView just emits the base view (no Delta wrap).

**Open sub-question:** should Delta's inner be restricted at the type
level (e.g. a `DeltaInner` marker interface)? My read: not worth it.
Document the constraint in the godoc, let pickView enforce.

### 3. Golden-test color format

Tests need to assert renderer output. lipgloss emits ANSI escape
sequences when its profile is set to TrueColor; emits plain text under
`lipgloss.SetColorProfile(termenv.Ascii)`.

**A. Mono only in goldens** — set ASCII profile in test setup; goldens
are plain text
✓ readable diffs, stable across terminals
✗ never tests color theme — color regressions slip through

**B. Color on, escape codes in goldens** — raw ANSI in `.golden` files
✓ catches every byte
✗ unreadable diffs; tooling chokes; slightest theme tweak rewrites N files

**C. Two golden suites** — mono goldens are plaintext, color goldens
record a *structural* form (e.g. `[red bold]error[/]` placeholders
produced by a tiny escape→tag reverse-mapper)
✓ both modes covered, both diffable
✗ 50 LOC of escape-sequence parsing we own forever

**D. Mono goldens + Color smoke test** — A plus a single hand-written
test per color-affected variant that asserts "output contains
`\x1b[38;5;196m`" (red FG) — no goldens, just presence checks
✓ catches "color theme silently dropped" without owning a parser
✗ doesn't catch "wrong shade of red"

**Recommendation: D.** Goldens are mono. Per-variant color smoke tests
assert a small set of escape-sequence presences (red for error, green
for pass, etc). Anything finer than "is this severity colored at all"
is a theme test, not a view test — and pkg/theme already owns those.

Test setup: each view test runs twice — once with `theme.Mono()`
asserting against `.golden`, once with `theme.Color()` asserting the
escape-presence checks.

## Proposed package layout

```
pkg/view/
  view.go         // ViewSpec marker interface, all variant structs
  render.go       // Render(ViewSpec, theme.Theme) string — the type switch
  clean.go        // renderClean (called from switch)
  bullet.go       // renderBullet, renderGrouped (share row helper)
  leaderboard.go  // renderLeaderboard
  headline.go     // renderHeadline, renderAlert (both single-line dominant)
  delta.go        // renderDelta (calls Render on Inner)
  multiples.go    // renderSmallMultiples
  view_test.go    // per-variant table tests, mono goldens + color smoke
  testdata/golden/
    clean.golden
    bullet_simple.golden
    bullet_with_fix.golden
    grouped_severity.golden
    leaderboard_top3.golden
    ...
```

Render signature:
```go
func Render(spec ViewSpec, t theme.Theme, width int) string
```
`width` = available terminal columns; views that need it (Leaderboard
bars, SmallMultiples grid) consume it, others ignore. Default 80 if
unknown.

## What this doc does NOT decide

- pickView heuristics (7f5.5) — thresholds already in epic notes; just
  needs implementation
- Streaming integration (7f5.7) — views render whole, channel-fed live
  mode emits one ViewSpec per tick
- CLI plumbing (7f5.8)

## Acceptance for 7f5.4

- All 8 variants render with sample data; per-variant golden files
  committed under `pkg/view/testdata/golden/`
- Each color-affected variant has a presence-check test asserting the
  expected severity escape appears in Color output
- Bullet and Grouped surface `FixCommand` when present
- `go test ./pkg/view/...` passes; `make check` clean
- No imports outside `pkg/paint`, `pkg/theme`, `pkg/report`,
  `unicode/utf8`, `strings`, stdlib

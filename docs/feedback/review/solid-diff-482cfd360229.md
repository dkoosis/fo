# SOLID review — diff 482cfd360229

Scope: cmd/fo (main, suppress, watch, watchkey), pkg/cluster, pkg/report
(filter, report), pkg/sarif/aggregates, pkg/scene, pkg/suppress (match,
suppress), pkg/testjson/toreport, pkg/view (scene_human, scene_llm),
pkg/wrapper/wraparchlint/archlint.

Note: target list referenced `pkg/testjson/stats.go` and
`pkg/wrapper/wraparchlint/convert.go`; neither exists in the tree. Skipped.

## Verdict

Overall: green-with-yellow. P1 LSP: green (no implementer stubs;
single-impl interfaces dominate). P1 SRP: yellow — two real candidates,
both in pkg/scene and pkg/suppress where header/attr tokenization is
welded to domain types. P2 ISP: green — no fat interfaces; the few
interfaces present (`io.Reader`/`io.Writer` consumers) are tight.

The new packages are unusually disciplined for surface design: small
types, narrow methods, no abstract base classes, no premature
interfaces. Findings below are design refinements, not bugs.

---

### 1. [F1] `pkg/scene/scene.go:341-405` — srp-mixed-concerns

**Symbol:** `pkg.scene` (file-level, `tokenizeAttrs` + `attrTokState`)
**Principle:** SRP

**Evidence:** scene.go does three jobs: (a) document parse (`Parse`,
`parser`, `feed*`), (b) act/actor/exit grammar
(`parseActHeader`/`parseActorLine`/`parseExitTrailer`), (c) shell-style
attr tokenization with quote/escape state machine (`tokenizeAttrs`,
`attrTokState.step{,InQuote,Bare}`).

The tokenizer in scene.go is a near-clone of `pkg/suppress/suppress.go`
`tokenize`/`tokState` (compare lines 182-238 there vs 341-405 here).
Comment at scene.go:340 even says "mirrors pkg/suppress conventions" —
that's the duplication confession. Two copies of the same state machine
in two packages.

**Code (smell):**
```go
// scene.go
type attrTokState struct{ inQuote, escape bool }
func (st *attrTokState) step(c byte, cur *strings.Builder, toks *[]string) error { ... }
func (st *attrTokState) stepInQuote(...)
func (st *attrTokState) stepBare(...)

// suppress.go (same shape, separately maintained)
type tokState struct{ inQuote, escape bool }
func (st *tokState) step(c byte, cur *strings.Builder, toks *[]string) error { ... }
```

**Fix:** extract a `pkg/kvtok` (or `internal/kvtok`) with one
`Tokenize(line string) ([]string, error)`. scene.go keeps document
parsing; suppress.go keeps suppression parsing; both depend on kvtok.
Closes one D11 (duplicate-via-divergence) hazard — the two will drift
on escape semantics.

---

### 2. [F2] `pkg/scene/scene.go:138-239` — srp-mixed-concerns

**Symbol:** `pkg.scene.parser`
**Principle:** SRP

**Evidence:** `parser` is a single struct holding state for four
sub-grammars: header (`headerSeen`, `feedHeader`), act framing
(`curAct`, `feedBody` case `## `), narration (`feedBody` case `> `),
and command-output capture (`curCmd`, `feedOutput`, `flushCmd`,
`isOutputLine`). 9 methods, four concerns.

The unit test surface bears it out: feed() switches on whether we are
in header mode, then on whether a command is open, then on the leading
sigil. Three layers of mode dispatch on one type.

**Fix:** split state by phase:
- `headerParser` owns `feedHeader` (returns when header consumed).
- `bodyParser` owns acts/narration/commands and the `curAct/curCmd`
  pair.

`Parse` becomes: run headerParser until it reports header-seen, hand
remaining lines to bodyParser. Reduces dispatch depth; makes each
phase's invariants visible at its type.

This is a yellow-band finding — the merged shape works and is small
enough. Worth fixing only if a third phase (e.g., epilogue) is
imminent.

---

### 3. [F3] `pkg/cluster/cluster.go:92-165` — srp-mixed-concerns

**Symbol:** `pkg.cluster` (`Run`/`RunWith` + `buildCluster` +
`mostCommon` + union-find)
**Principle:** SRP

**Evidence:** cluster.go (306 LOC) merges five concerns:
1. Config defaults (`Config`, `withDefaults`).
2. Pipeline orchestration (`Run`, `RunWith`).
3. Mode-specific union strategy (`unionBy` + switch on `Mode`).
4. Cluster assembly (`buildCluster`, `mostCommon`).
5. Union-find data structure (`unionFind`, `newUnionFind`, `find`,
   `union`).

The union-find is generic; it has no domain reference and could live
in a sibling file or `internal/disjoint`. Cluster assembly post-union
is also self-contained. Keeping all five in one 306-line file
penalizes change locality — the next person tweaking mode semantics
walks past union-find internals to reach the switch.

**Fix:** keep cluster.go as the orchestrator (Config, Run, RunWith).
Split out:
- `pkg/cluster/unionfind.go` — `unionFind` + methods.
- `pkg/cluster/build.go` — `buildCluster`, `mostCommon`, `record`.

Pure file-level move; no API change. Earns its keep when mode #5
arrives or union-find grows path-splitting.

---

### 4. [F4] `pkg/scene/scene.go:53-66` — isp-caller-uses-subset (latent)

**Symbol:** `pkg.scene.Beat`
**Principle:** ISP / D5 (union-via-flat-struct)

**Evidence:** `Beat` carries `Kind BeatKind` plus both `Narration
string` and `Command Command`. Renderers must switch on Kind and read
exactly one of the other two fields. Both view files do this:

```go
// scene_human.go, scene_llm.go
switch beat.Kind {
case scene.BeatNarration: renderNarration(w, beat.Narration, t)
case scene.BeatCommand:   renderCommand(w, beat.Command, t)
}
```

The struct lets a caller construct a Beat with `Kind=BeatCommand`
*and* a non-empty `Narration` — an invalid state the type permits.

**Fix (defer, not now):** `Beat` as an interface with
`isBeat()`/visitor, or a sealed sum via private constructor returning
`any` plus type-switch. Either kills the impossible-state hazard.

Don't flag for action now: only two beat kinds exist, both renderers
are 100-line files, and an interface for two cases is over-design at
this size. Re-evaluate when a third kind (e.g., `BeatPause`) lands.

---

### 5. [F5] `cmd/fo/main.go:1136-1304` — srp-mixed-concerns (package level)

**Symbol:** `main` package — `runWrap` dispatch + per-wrapper glue
**Principle:** SRP

**Evidence:** cmd/fo/main.go is 1316 lines and 40+ top-level
functions. `runWrap` is a switch over wrapper names that imports every
wrapper package and calls its `Convert`. Adding a wrapper edits both
the switch in main and the package list.

The CLAUDE.md acknowledges this — "Dispatched by `switch` in
`cmd/fo/main.go` (no interface, no registry)" is policy, not
oversight. So this is documented green. Flagged only to note that the
file is on the edge: at ~1300 lines, the next 2-3 features push it
past comfortable.

**Fix (when triggered):** extract `cmd/fo/wrap.go` for runWrap and its
helpers; extract `cmd/fo/sniff.go` for the `sniff*` family
(`sniffSARIF`, `sniffGoTestJSON`, `sniffBareTally`,
`looksLikeLineDiagnostics`, `hasJSONShapedLine`). Pure file split.

Not actionable this diff. Recorded as a pressure reading.

---

## Skipped

- OCP: out of scope per linter spec.
- DIP: deferred to `/review arch`.
- LSP-stub-impl: searched `panic(`, `errors.New("not"`, `ErrUnsupported`
  across the diff — no hits in production paths. Watch files'
  `restoreTTY := func() {}` no-op is the documented non-TTY fallback,
  not an interface stub.
- Fat interfaces: largest interface in scope is `io.Reader` / Closer
  composite use; no project-defined interface ≥4 methods appears in
  these files.

Accept-ratio note: 2 findings (F1, F3) are concrete fix-now;
3 are watchlist (F2, F4, F5).

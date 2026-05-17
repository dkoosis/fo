# Domain Vocabulary Review — fo

Run: `bd775e303d86-domain-vocab` · Target: repo · Mode: report · Cap: 10

Focus: call-site readability. Boolean traps, inline func-type repetition, vocab drift.
Pairs with `/review truthful-names` (per-symbol accuracy).

---

### 1. [F1] `pkg/theme/theme.go:142` — bool-trap-exported-api

**Diagnosis.** `theme.Default(stdoutIsTTY bool) Theme` is an exported, single-bool API. Call sites read `theme.Default(true)` / `theme.Default(false)` with no clue what the bool selects.

**Why.** The parameter name `stdoutIsTTY` is informative; the call site discards it. A reader of `theme.Default(true)` in `pkg/view/scene_human.go:40` cannot tell whether `true` means "use color", "force TTY", "isatty=true", or something else without jumping to the definition.

**Evidence (Read-verified).**
- Definition: `pkg/theme/theme.go:142` — `func Default(stdoutIsTTY bool) Theme`.
- `pkg/view/scene_human.go:40` — `t := theme.Default(true)`.
- `cmd/fo/main.go:877` — `return theme.Default(isTTYWriter(w))` (self-documenting; the bare-literal sites are not).
- `pkg/theme/theme_test.go:49,57,65` — `theme.Default(true)` / `theme.Default(false)`.
- `pkg/view/scene_human_test.go:77` — comment literally says `should use theme.Default(true)` — readers need a paren-aside to discuss the bool.

**Fix.** Two viable shapes:
1. Split into named selectors: `theme.ForTTY()` and `theme.ForPipe()` (or `theme.Color()` + `theme.Mono()`); keep `Default(isTTY bool)` as the lower-level builder.
2. Replace the bool with a typed mode: `type Surface int; const (SurfaceTTY Surface = iota; SurfacePipe); func Default(s Surface) Theme`.

Either makes `theme.Default(true)` → `theme.ForTTY()` / `theme.Default(SurfaceTTY)`.

**Tier.** 🔴 (exported API · ≥3 bare-literal call sites across render + test code).

---

### 2. [F2] `pkg/cluster/frame.go:74` — bool-trap-exported-api (intra-pkg propagation)

**Diagnosis.** `keepAbsPaths bool` threads through `extractTopUserFrame`, `scanPanicStack`, `matchCite`, renamed to `keepAbs` in `formatFrame`. Test call sites are bare `true`/`false`.

**Why.** The bool propagates unchanged through four helpers. Production carries it as `cfg.KeepAbsPaths` (fine), but **test call sites** lose the name: `extractTopUserFrame(out, false)` and `extractTopUserFrame(out, true)` read as identical noise.

**Evidence (Read-verified).**
- Declarations: `pkg/cluster/frame.go:74, 87, 117, 129`.
- Test sites: `pkg/cluster/frame_test.go:16, 28, 40, 52, 61, 69, 76` — 7 bare-literal calls.
- Production site `pkg/cluster/cluster.go:179` reads `cfg.KeepAbsPaths` (self-documenting).

**Fix.** Promote to a typed mode local to this file: `type PathStyle int; const (PathTrimmed PathStyle = iota; PathAbsolute)`. Test calls become `extractTopUserFrame(out, PathTrimmed)`. Rename the struct field to `cfg.PathStyle` for parity.

**Tier.** 🔴 (4 propagating signatures + 7 bare-literal test call sites).

---

### 3. [F3] `pkg/cluster/cluster.go:185,238` — inline-func-type-repeated

**Diagnosis.** `func(Signals) string` appears 6× across `cluster.go` as parameter type and inline closure. It is a real domain concept — "extract a clustering key from one record's signals" — with no name.

**Why.** Per rule `inline-func-type-repeated`: identical inline types in ≥2 places should be aliased once. Naming this also documents the contract: keys are derived from `Signals` alone, not the full record.

**Evidence (Read-verified).**
- Param types: `unionBy(..., key func(Signals) string)` (`cluster.go:185`); `mostCommon(..., pick func(Signals) string)` (`cluster.go:238`).
- Inline closures: `cluster.go:129, 130, 135, 142, 144, 204, 205` — all of shape `func(s Signals) string { return s.SomeField }`.

**Fix.**

```go
// SignalKey extracts a clustering key from a record's signals.
type SignalKey func(Signals) string

func unionBy(uf *unionFind, recs []record, key SignalKey) { ... }
func mostCommon(members []int, recs []record, pick SignalKey) string { ... }
```

**Tier.** 🟡 (≥2 declaration sites, unexported · still worth aliasing for vocabulary).

---

### 4. [F4] `cmd/fo/main.go:902,911,1024` — bool-trap-multi-arg

**Diagnosis.** `runStream`, `runStreamCtx`, and `runStreamBatch` each take `noState, stateStrict bool` as adjacent positional params. Polarity is inverted (`noState` = don't write; `stateStrict` = error if write failed), compounding the trap.

**Why.** Two adjacent bools is the canonical bool trap. Inside the streaming goroutine (`cmd/fo/main.go:950`) the call is `attachDiff(r, stateFile, noState, stderr)` where `noState` is a plain local bool param — the named-flag-pointer mitigation that exists at the top-level call site is gone.

**Evidence (Read-verified).**
- Definitions: `cmd/fo/main.go:902, 911, 1024` — all `..., noState, stateStrict bool, stderr io.Writer`.
- `attachDiff` declaration: `cmd/fo/state.go:18` — `noState bool` as its own bool-trap-adjacent peer.
- Call sites: `cmd/fo/main.go:266, 268, 323, 950, 1034`.

**Fix.** Bundle into a `StateOpts` struct:

```go
type StateOpts struct {
    Path   string // sidecar path
    Off    bool   // skip read+write
    Strict bool   // exit 2 on save error
}
```

Then `runStream(stdin, br, stdout, t, stateOpts, stderr)` and `attachDiff(r, stateOpts, stderr)`. Flag block in `main` builds `StateOpts` once; every downstream signature shrinks.

**Tier.** 🔴 (≥3 funcs share the trap · streaming entry-point shape · polarity-inverted bools).

---

### 5. [F5] `cmd/fo/main.go:360` — vocab-drift (parameter naming)

**Diagnosis.** `renderHygiene(stdout, stderr io.Writer, mode string, ...)` uses `mode` for what is unambiguously a **format** elsewhere in the file (`formatJSON`, `formatLLM`, `formatHuman`; flag `--format`; resolver `resolveFormat`).

**Why.** The package consistently calls this concept "format". `pkg/view` separately owns a `Mode` type (`view.Mode`, `ModeHuman`) for a different axis (PickView dispatch). Using `mode string` in `cmd/fo` muddies the boundary — a reader of `renderHygiene(..., mode, ...)` may reasonably think `view.Mode` flows in, when it's the format string.

**Evidence (Read-verified).**
- Constants `cmd/fo/main.go:62-64`: `formatHuman`, `formatLLM`, `formatJSON`.
- Flag `cmd/fo/main.go:219`: `formatFlag := fs.String("format", ...)`.
- `renderHygiene` switch (`cmd/fo/main.go:362, 369`) cases `formatJSON`, `formatLLM` — parameter still named `mode`.
- Sibling helpers carry the same drift: `renderTally(..., mode, themeName string)` (`cmd/fo/main.go:388`); `renderStatus(..., mode string)` (`cmd/fo/main.go:431`); `renderScene` (`cmd/fo/main.go:416`).

**Fix.** Mechanical rename of the parameter `mode` → `format` across `cmd/fo/main.go` hygiene helpers. Reserve `mode` for `view.Mode` values. No semantic change.

**Tier.** 🟡 (single-package drift; ~6 helper signatures · cosmetic but persistent).

---

### 6. [F6] `pkg/view/stream.go:83` — bool-trap-multi-arg (latent · `*bool` sentinel)

**Diagnosis.** `writeSnapshot(w io.Writer, r report.Report, t theme.Theme, width int, first *bool, mode Mode) error` takes an in-band `first *bool` flag. The call shape `writeSnapshot(w, r, t, width, &first, mode)` reads like a state-machine knob the reader must trace.

**Why.** `*bool` for "is this the first emission?" is a recurring antipattern: the bool gets set after first use, mutates caller state, and the call site cannot tell if `&first` is input, output, or both. This is the bool-trap pattern applied to a sentinel.

**Evidence (Read-verified).**
- Definition: `pkg/view/stream.go:83`.
- Call sites: `pkg/view/stream.go:63, 73`, both passing `&first` from a local `first := true` in `RenderStreamMode` (`pkg/view/stream.go:53`).
- Role of bool: emit a separator newline before all snapshots except the first.

**Fix.** Lift the separator concern out of `writeSnapshot`:
1. Make `writeSnapshot` return the rendered string; caller prepends `\n` when needed.
2. Or introduce a tiny stateful `snapshotWriter` in `RenderStreamMode` that owns the first-flag internally.

Either removes the `*bool` from the signature.

**Tier.** 🟡 (single function, internal · flag before the pattern spreads).

---

## Summary

6 findings (under cap). Two 🔴: `theme.Default` exported bool trap and the `noState/stateStrict` pair across `runStream*`. One 🟡 repeated inline-func-type in `pkg/cluster`. Three 🟡 vocabulary cleanups.

No findings for: `lineread`'s `oversize bool` (return value, not a call-site argument); `watchLoop`'s `runOnce func()` / `between func()` (zero-arg, self-documenting); `pickHeadline`/`pickAlert`/etc. returning `(T, bool)` (idiomatic Go ok-pattern).

# Domain Vocabulary Review — fo

Run: `7858b3a8ea1b` · Target: repo (whole codebase) · Mode: report · Cap: 10

Focus: call-site readability — boolean traps, inline func-type repetition, vocab drift.
Pairs with `/review truthful-names` (per-symbol accuracy).

---

### 1. [F1] `pkg/cluster/cluster.go:180,233` — inline-func-type-repeated

**Diagnosis.** `func(Signals) string` is the parameter type of both `unionBy` (`cluster.go:180`) and `mostCommon` (`cluster.go:233`), and recurs as an inline closure literal at 7 call sites. It's a real domain concept — "derive a clustering key from one record's signals" — with no name of its own.

**Why.** Per `inline-func-type-repeated`: an identical inline type appearing in ≥2 places should be aliased once. Here it's 2 parameter declarations plus 7 closures, all shaped `func(s Signals) string { return s.<Field> }` or a short derivation. A reader has to reconstruct "key extractor over Signals" from repetition instead of reading it off a type name.

**Evidence (Read-verified).**
- `pkg/cluster/cluster.go:180`: `func unionBy(uf *unionFind, recs []record, key func(Signals) string) {`
- `pkg/cluster/cluster.go:233`: `func mostCommon(members []int, recs []record, pick func(Signals) string) string {`
- Closures of the same shape: `cluster.go:129` `func(s Signals) string { return s.TopUserFrame }`, `:130` `func(s Signals) string { return s.NormSig }`, `:135-140` (multi-line tuple-key variant), `:142`, `:144`, `:199`, `:200`.

**Fix.**
```go
// SignalKey extracts a clustering key from a record's signals.
type SignalKey func(Signals) string

func unionBy(uf *unionFind, recs []record, key SignalKey) { ... }
func mostCommon(members []int, recs []record, pick SignalKey) string { ... }
```
No call-site change needed — closures still satisfy the named type.

**Tier.** 🟡 (2 declaration sites + 7 closures, unexported — still worth aliasing for vocabulary; this is the one domain-vocab item from the prior repo pass that wasn't addressed in the `fo-pii` cleanup round, unlike the sibling `PathMode`/`statePolicy` fixes).

---

### 2. [F2] `pkg/view/stream.go:99` — bool-trap-multi-arg

**Diagnosis.** `flushStream(w io.Writer, pendingClean *report.Report, t theme.Theme, width int, first *bool, mode Mode, rendered bool) error` carries two bool-shaped parameters — `first *bool` (a mutable "is this the first snapshot" sentinel) and `rendered bool` (input, "did a snapshot already render") — through the same call.

**Why.** The signature reads like a state machine the caller must trace to use correctly: `first` is threaded by pointer so it can be *mutated* by a callee three frames down (`flushStream` → `writeSnapshot`), while `rendered` is a plain input bool computed by the caller's own loop. At the one call site (`RenderStreamMode`, `stream.go:68`) both feed from local booleans (`first`, `rendered`) that the reader must already be tracking to tell them apart — exactly the ambiguity `bool-trap-multi-arg` calls out, compounded here by one of the two being a pointer with hidden write-back semantics.

**Evidence (Read-verified).**
- Definition: `pkg/view/stream.go:99` — `func flushStream(w io.Writer, pendingClean *report.Report, t theme.Theme, width int, first *bool, mode Mode, rendered bool) error {`
- Call site: `pkg/view/stream.go:68` — `return flushStream(w, pendingClean, t, width, &first, mode, rendered)` inside `RenderStreamMode`.
- `first` originates as a plain `first := true` local at `stream.go:59`; `rendered` originates as `rendered := false` at `stream.go:61`, flipped via `streamStep.rendered` from `handleSnapshot` (`stream.go:75-77`).
- The codebase already has the fix pattern in hand one struct over: `streamStep` (`stream.go:82-85`) bundles `pending *report.Report` + `rendered bool` on the *return* side of `handleSnapshot`, precisely to avoid an ambiguous multi-value return. The same treatment hasn't been applied to `flushStream`'s *input* side.

**Fix.** Fold `first` and `rendered` into a small stream-state value the loop owns and passes by value or via one pointer, e.g.:
```go
type streamState struct {
    first    bool
    rendered bool
}
func flushStream(w io.Writer, pendingClean *report.Report, t theme.Theme, width int, st *streamState, mode Mode) error {
    if pendingClean != nil && !st.rendered {
        return writeSnapshot(w, *pendingClean, t, width, &st.first, mode)
    }
    return nil
}
```
(`writeSnapshot`'s own `first *bool` can fold into the same struct in a follow-up if desired — not required to fix this call site.)

**Tier.** 🟡 (single call site, unexported — low blast radius, but the mutable-pointer-bool + plain-bool pairing is the shape the rule exists to catch, and the fix is a small, low-risk mechanical change).

---

## Not flagged (checked, cleared)

- `cmd/fo/main.go:389` `resolveStatePolicy(noState, strict bool)` — two bool params, but this *is* the CLI-flag→enum boundary conversion (bead `fo-pii`); it's called from exactly one site with named flag vars, and immediately produces the typed `statePolicy` enum consumed everywhere else. This is the fix, not the trap.
- `pkg/cluster/frame.go` `keepAbsPaths` — already isolated to one `pathMode(keepAbs bool) PathMode` conversion (comment at `frame.go:68` says so explicitly); `PathMode`/`PathTrim`/`PathKeep` used everywhere else, including tests.
- `pkg/theme/theme.go` `Default` — already takes a typed `OutputKind` (`OutputTTY`/`OutputPipe`), not a bool.
- `paint.Columnize(rows, 2)` / `(..., 3)` — bare int `gap` recurs across `pkg/view/{bullet,leaderboard,multiples}.go`, but the doc comment on `Columnize` names the unit ("cells are joined with `gap` spaces") and the 2-vs-3 split tracks a real visual distinction (leaderboard rows vs. grid cells), not drift.
- Named constants throughout `pkg/state` (`MaxHistory`, `MaxRunLog`, `MaxMetricsHistory`), `internal/boundread.DefaultMax`, `sarif.maxNestingDepth`, and `testjson`'s `panicBoost`/`buildErrorBoost` — no magic literals at call sites.
- Wrapper packages (`pkg/wrapper/wrap*`) already use `Opts`/`DiagOpts` structs instead of positional args.
- Exported-symbol vocabulary walked package-by-package (`cluster`, `sarif`, `testjson`, `view`, `theme`, `paint`, `report`, `state`, `status`, `tally`, `metrics`, `hygiene`, `multiplex`, `suppress`, `scene`, `fingerprint`, `score`) — each surface reads consistent with its stated purpose; no misplaced concern found.
- `cmd/fo/render.go`'s `renderHygiene`/`renderTally`/`renderStatus`/`renderScene`/`renderMetrics` all name their format-string parameter `mode`, while `pkg/view` separately owns a distinct `Mode` type for view dispatch — a real naming collision across the two packages, but it doesn't cleanly match any rule id in `domain-vocab.rules.md` (closest is `identifier-outside-pkg-vocabulary`, which targets a symbol foreign to its *own* package's vocabulary, not a name reused for a different concept in a neighboring package). Noted here rather than forced into a mismatched rule id; worth a mechanical `mode`→`format` rename in `cmd/fo` if raised again as a `truthful-names` finding.

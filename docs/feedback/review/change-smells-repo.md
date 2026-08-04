# change-smells — repo

RUN_ID: 7858b3a8ea1b
Scope: /Users/vcto/Projects/fo (whole repo)
History window: 6 months, 404 commits (well above 30-commit floor).

## Result: 0 findings

A prior run of this linter (`RUN_ID bd775e303d86`, report still in git
history) raised three findings against `cmd/fo/main.go` and `pkg/wrapper/*`.
Re-mining the same rule catalog now shows two of those three are fully
resolved and the third is partially resolved with no remaining git-history
evidence — nothing new clears the bar this pass.

## Prior findings, re-verified

**F1 (divergent-change, `cmd/fo/main.go`) — resolved.** The prior report's
exact suggested split (`dispatch.go`, `parse.go`, `render_hygiene.go`,
`stream.go`, `state_cmd.go`, `wrap_cmd.go`) landed via
`refactor(cmd/fo): split main.go by verb cluster (fo-ajw)` (2026-06-15).
`cmd/fo/main.go` is now 477 lines (was 1339); its dispatch verbs live in
`cmd/fo/{parse,render,stream,state,wrap,watch,explain,suppress_cmd,trend}.go`.
Commit count on `main.go` since the refactor: 2 (both the same feature,
`fo-w1f`, tee-to-sidecar-log) — no divergence, single axis.

**F2 (data-clumps, `runStream`/`runStreamCtx`/`runStreamBatch`) — resolved.**
The prior report's exact suggested fix — collapse the trailing
`(stateFile, noState, stateStrict, stderr, …)` clump into one struct — landed.
`cmd/fo/stream.go:36`:
```go
type streamOpts struct {
	stdin     io.Reader
	br        *bufio.Reader
	stdout    io.Writer
	stderr    io.Writer
	theme     theme.Theme
	themeName string // only used by runStreamBatch's deferred renderMode
	mode      string // only used by runStreamBatch
	stateFile string
	policy    statePolicy
}
```
`cmd/fo/stream.go:52`: `func runStream(opts streamOpts) int`; `:61`
`func runStreamCtx(ctx context.Context, opts streamOpts) int`; `:178`
`func runStreamBatch(opts streamOpts) int`. Single param, no positional
bool-pair foot-gun.

**F3 (shotgun-surgery, `pkg/wrapper/*` triad) — partially resolved; no
current git-history evidence.** The prior report flagged
`wraparchlint.go ↔ wrapjscpd.go ↔ wrapdiag/*` co-changing 8/6/5 times and
suggested (a) collapsing `cmd/fo`'s dispatch-by-switch into a registry, and
(b) a generic `wrapper.Run[T]` helper to dedupe each `Convert`'s
`boundread.All → parse → build` body.

(a) landed: `cmd/fo/wrap.go:43-50` now holds a `plainWrappers` map
(`name → {flagSet, convertFunc}`); adding a flagless wrapper is a one-line map
entry plus a `wrapNames`/`wrapDescriptions` update in the same file, not an
edit to `cmd/fo/main.go`.

(b) did not land — `wraparchlint/archlint.go:17` and `wrapjscpd/jscpd.go:32`
still each call `boundread.All(r, 0)` directly; no shared runner exists. But
re-running the co-change mining restricted to `--since=2026-06-15` (the
registry-refactor date) returns **zero pairs at any threshold ≥2** among the
wrapper packages — none of them has been touched together (or separately)
since the registry landed. Under this linter's own evidence bar ("smells only
derivable from git history or the call graph"), the remaining duplication is
no longer a *change-coupling* smell — it's a static-duplication smell with no
current co-change evidence, which is `dedup-patterns`' territory, not this
linter's. Not re-flagged here.

## Fresh mining, all rules

- **Feature Envy** (SQL over `call_graph`, callee-pkg calls outnumbering
  own-pkg, ≥3): two candidates clear the raw floor and both fail on
  inspection. `cmd/fo.Close` → `pkg/testjson`: 3 calls, but also 3 calls into
  its own package — ties, doesn't *outnumber*. `wrapjscpd.Convert` →
  `pkg/sarif`: 3 calls, but building a `pkg/sarif` result is that adapter's
  entire job (the documented wrapper pattern in
  `.claude/rules/CLAUDE.md` §Key Design Decisions) — the rules catalog's own
  "orchestration layer" exemption. 0 findings.
- **Inappropriate Intimacy** (mutual imports): checked all 93 edges in
  `snipe deps --tree` for A→B/B→A pairs. None found.
- **Message Chains** (`\w+()\.\w+()\.\w+()`, ≥3 links): `rg` over all
  non-test `.go` files, zero matches.
- **Data Clumps** (same 3+ param names, ≥4 signatures): two clumps clear the
  raw floor. `{w, tool, rows}` across `RenderStatus{LLM,Human}` /
  `RenderMetrics{LLM,Human}` (`pkg/view/status.go`, `pkg/view/metrics.go`) is
  the intentional human/llm-peer-renderer pairing the north-star calls out
  ("Two readers, one IR") — not a missing domain type, just two renderers per
  shape by design. `{args, stdin, stdout, stderr}` across `run`, `runWatch`,
  `runWrap`, `runWrapDiag`, `runWrapLeaderboard` is the standard Go CLI
  stdin/stdout/stderr-injection idiom, constructed at each call site from
  disparate origins (`os.Args`, `os.Stdin`, …) — the rules catalog's own
  "constructed from disparate origins" exemption. 0 findings.
- **Primitive Obsession** (≥30% exported funcs on bare `string`, ≥3 missing
  domain types): no exported function in the repo takes 3+ `string`
  parameters positionally; multi-string config already goes through option
  structs (e.g. `wrapdiag.DiagOpts{Tool, Rule, Level, Version}`). 0 findings.

## Scoring

| Tier | Verdict |
|------|---------|
| P1 Change (shotgun-surgery, divergent-change) | 🟢 0 — both prior hits resolved by the fo-ajw refactor |
| P1 Couplers (feature-envy, inappropriate-intimacy, message-chains) | 🟢 0 |
| P2 (data-clumps, primitive-obsession) | 🟢 0 |

Nothing to change this pass. Kept the resolution trail above so a future run
doesn't re-litigate the same history — F1/F2 are done, F3's registry half is
done and its dedup half has no live change-coupling evidence under this
linter's rules (route it through `dedup-patterns` if picked up again).

# change-smells — repo

RUN_ID: bd775e303d86-change-smells
Scope: /Users/vcto/Projects/fo (whole repo)
History window: 6 months, 678 commits (well above 30-commit floor).
Note: many pre-v2-cutover files (`fo/console.go`, `mageconsole/console.go`, `cmd/main.go`, `pkg/dashboard/*`, `pkg/design/*`, `pkg/render/llm.go`, `pkg/mapper/*`) appear in raw co-change pairs but no longer exist in `git ls-files`; filtered out. Findings below restricted to currently-tracked Go files.

Overall tier: 🟡 — 1 strong divergent-change (cmd/fo/main.go), 1 data-clump (run-pipeline IO+state params), 1 shotgun-surgery cluster (wrapper triad), 0 feature-envy after orchestration-exception filter, 0 inappropriate-intimacy, 0 message-chains, 0 primitive-obsession.

---

### 1. [F1] `cmd/fo/main.go:1` — divergent-change

Diagnosis: `cmd/fo/main.go` (1339 LOC) has accrued 89 distinct commit subjects in 6mo spanning ≥5 unrelated verb-clusters: CLI flag/help (`feat(cli)`, `feat(wrap)`), parse-ladder (`feat(parse)`, `fix(parse)`), stream/state (`fix(stream)`, `fix(state)`), wrap dispatch (`refactor(wrapper)`), hygiene render (`refactor(cmd/fo): extract renderHygiene`). The file owns: subcommand dispatch, format sniffing, parse fallback ladder, hygiene render fan-out, stream pipeline, watch wiring, state CLI — seven axes of change in one file.

Why: divergent-change costs every contributor a re-read of unrelated code on each unrelated edit, inflates merge-conflict surface across parallel work, and the file is the hub of every cross-cutting shotgun pair below.

Evidence: Read-verified. `cmd/fo/main.go` defines (line ranges): `run` L186, `renderMode` L335, `renderHygiene` L360, `renderTally/Scene/Status/Metrics` L388-L597, `parseToReport/parseTestJSONTolerant/parseMultiplex/parseSection` L597-L902, `runStream/runStreamCtx/runStreamBatch/runTestJSONPipeline` L902-L1117, `runState` L1117, `runWrap` L1159. Git verb tally for this path (6mo): 9 `feat(cli)`, 8 `fix`, 7 `feat`, 5 `refactor(fo)`, 4 `feat(wrap)`, 3 `refactor(cmd/fo)`, 3 `fix(stream)`, 2 each {`simplify`, `refactor(stream)`, `refactor(render)`, `fix(state)`, `fix(lint)`}.

Fix: split along verb-clusters. Suggested partition (each becomes its own file in `cmd/fo/`, all package `main`):
- `dispatch.go` — `run`, subcommand fan-out, `--version`/`--print-schema`.
- `parse.go` — `parseToReport`, `parseTestJSONTolerant`, `parseMultiplex`, `parseSection`, sniffers.
- `render_hygiene.go` — `renderHygiene`, `renderTally/Scene/Status/Metrics`, `coerceAs`.
- `stream.go` — `runStream`, `runStreamCtx`, `runStreamBatch`, `runTestJSONPipeline`, `sendCoalesceSnapshot`.
- `state_cmd.go` — `runState`.
- `wrap_cmd.go` — `runWrap`.

No API churn — purely organizational. Pairs naturally with F2.

Tier: P1 — change preventer.

---

### 2. [F2] `cmd/fo/main.go:902` — data-clumps

Diagnosis: The quadruple `(stateFile string, noState, stateStrict bool, stderr io.Writer)` recurs as a parameter clump across `runStream` (L902), `runStreamCtx` (L911), `runStreamBatch` (L1024). Combined with the IO trio `(stdin io.Reader, br *bufio.Reader, stdout io.Writer)`, these three functions each carry 8 positional parameters of which 6 are identical between callers.

Why: data-clumps make call sites brittle (the positional `noState, stateStrict` bool pair is easy to swap silently), defeat go-vet help, and force any new state knob (e.g., a sidecar-path override) to ripple through all three signatures plus their tests.

Evidence: Read-verified. `cmd/fo/main.go` L902:
```go
func runStream(stdin io.Reader, br *bufio.Reader, stdout io.Writer, t theme.Theme, stateFile string, noState, stateStrict bool, stderr io.Writer) int {
```
L911 `runStreamCtx` adds `ctx context.Context` prefix; L1024 `runStreamBatch` swaps `t theme.Theme` for `mode, themeName string`. The quad `(stateFile, noState, stateStrict, stderr)` is forwarded verbatim at L905: `return runStreamCtx(ctx, stdin, br, stdout, t, stateFile, noState, stateStrict, stderr)`.

Fix: introduce a `streamOpts` struct in the same file:
```go
type streamOpts struct {
    stateFile string
    noState   bool
    strict    bool          // was stateStrict — name is now unambiguous in context
    theme     theme.Theme   // Ctx path
    themeName string        // Batch path
    mode      string        // Batch path
}
```
Then `runStream(stdin, br, stdout, stderr io.Writer, opts streamOpts) int` and the inner ctx variant just take `opts` plus `ctx`. Drops 6→1 trailing args per signature and kills the bool-pair foot-gun. Keep the IO trio positional — it's a stable Go idiom.

Tier: P2 — data smell.

---

### 3. [F3] `pkg/wrapper/*` — shotgun-surgery

Diagnosis: The wrapper triad `pkg/wrapper/wraparchlint/archlint.go ↔ wrapjscpd/jscpd.go ↔ wrapdiag/convert.go` (and siblings) co-changes 8+6+5 times, each commit spanning 3+ packages. Recurring shared-axis commits: `fo-s5x: bound wrapper io.ReadAll via boundread helper`, `fo-ffy: per-wrapper FixCommand generators`, `fo-op6: stream stdin incrementally`, `fo-gn0/18j: bounded line reader + injected stderr`, `refactor(wrapper): split Wrap into RegisterFlags+Convert`, `feat(wrapper): add descriptions to registry, generate help dynamically`, `fix: strict-lint pass`, `fix: silence unused-parameter lint in wrapjscpd and wraparchlint`.

Why: each cross-cutting wrapper concern (bounded reads, FixCommand, registry metadata, lint posture) currently lives copy-pasted across N adapter files. The shape "add field X to every wrapper" is a 6-file edit at minimum. The CLAUDE.md canonizes the pattern: *"Adding a wrapper: new package under pkg/wrapper/, expose Convert, add a case to the wrap dispatch + import in cmd/fo/main.go"* — the design explicitly chose duplication over a registry, and the cost is showing in the co-change graph.

Evidence: Read-verified. `pkg/wrapper/` contains 7 sibling adapter pkgs (`wraparchlint`, `wraparchlinttext`, `wrapcover`, `wrapdiag`, `wrapgobench`, `wrapjscpd`, `wrapleaderboard`). All expose `func Convert(in io.Reader, out io.Writer) error`. `wrapjscpd/jscpd.go` L32 reads `boundread.All(r, 0)`; `wraparchlint`, `wrapdiag`, `wrapgobench` do the same wrapper-boundread dance. Co-change pairs (current-files-only, last 6mo): `wraparchlint/archlint.go ↔ wrapjscpd/jscpd.go` ×8, `cmd/fo/main.go ↔ wrapdiag/diag.go` ×7, `wrapdiag/diag.go ↔ wrapjscpd/jscpd.go` ×6, `cmd/fo/main.go ↔ wrapjscpd/jscpd.go` ×5, `wraparchlint/archlint.go ↔ wrapdiag/diag.go` ×5. Span: 3-4 pkgs per change.

Fix: introduce a thin shared helper in `pkg/wrapper/` (sibling of the `doc.go` already there):
```go
// pkg/wrapper/runner.go
type ParseFn[T any] func([]byte) (T, error)
type BuildFn[T any] func(T, io.Writer) error

func Run[T any](r io.Reader, w io.Writer, parse ParseFn[T], build BuildFn[T]) error {
    data, err := boundread.All(r, 0)
    if err != nil { return fmt.Errorf("reading input: %w", err) }
    v, err := parse(data)
    if err != nil { return err }
    return build(v, w)
}
```
Each wrapper's `Convert` becomes `return wrapper.Run(r, w, parseClones, buildSARIF)`. Future cross-cutting concerns (FixCommand templates, stderr injection, max-bytes override) get added once. The dispatch-by-switch in `cmd/fo/main.go` stays — this fix only dedupes the *body* of each `Convert`.

Don't-flag check: this is not "test-and-impl pair" or "early-stage pkg" — bulk of these co-changes are cross-cutting infrastructure rolls across all wrappers in one commit. That's the shotgun signature.

Tier: P1 — change preventer (≥3 pairs, spans ≥3 pkgs).

---

### Considered & dismissed

- **feature-envy: `wrapjscpd.Convert` → `pkg/sarif` (3 calls vs 1 own-pkg)**. Falls under explicit exception: *"methods that legitimately compose multiple pkgs as part of an orchestration layer"*. A wrapper's job *is* to convert format A into sarif builder calls; that's adapter-to-target, not envy. Same logic exempts `cmd/fo.Close → pkg/testjson` (3 vs 3, not even a majority).
- **inappropriate-intimacy**: SQL on `imports` table returns 0 mutual-import pkg pairs under `github.com/dkoosis/fo/`. Go's import-cycle rule plus codebase discipline keep this clean.
- **message-chains**: zero `a.b().c().d()` matches across non-test Go files.
- **primitive-obsession**: `cmd/fo` passes bare `string` for `mode`, `themeName`, `stateFile`, `kind`, `tool` (10+ sites), but the broader codebase already defines `report.Severity`, `report.TestOutcome`, `state.Severity`, `state.Class`, `status.State`, `cluster.ClusterID`, `testjson.Status` — domain types exist where they buy safety. The cmd-layer strings are CLI-flag values, validated once at parse time, then routed by switch; promoting them to named types would be ceremony without payoff. Doesn't meet the ≥3-distinct-domain-concepts-where-mixups-are-silent bar.
- **testjson parser ↔ types (7 co-changes)**: same-pkg pair, single concept (event parsing + its data shape co-evolve). Excluded by spec: *"pairs that co-change because they're literally the same feature being built"*.

### Cross-references

- File-size / complexity of `cmd/fo/main.go` (1339 LOC) is arch/clarity territory — see `arch-repo.md` and the clarity review for size-axis treatment of the same file. F1 here is strictly the *verb-spread* axis.
- The wrapper-dedup fix (F3) overlaps with what `simplify-flow` / DRY tooling would suggest; this report scopes only the change-coupling argument, not the duplication argument.

# change-smells — repo

run_id: f62c7fc3af14
scope: project (`/Users/vcto/Projects/fo`)
window: 6 months (613 commits)
module: `github.com/dkoosis/fo`

Notes:
- Snipe bundle (`/tmp/snipe-bundle-f62c7fc3af14/`) was empty/broken (`0 packages, 0 edges` in deps-tree; context-full unreadable). Call-graph SQL phases (feature-envy, inappropriate-intimacy) ran on live source only via grep heuristics.
- Heavy historical churn references files that no longer exist (`fo/console.go`, `pkg/design/*`, `pkg/dashboard/*`, `pkg/mapper/*`, `pkg/render/*`, `mageconsole/`) — pre-cutover noise. Findings filtered to current tree only.

Tier roll-up: P1-change 🟡 · P1-couplers 🟢 · P2 🟡

## F1

### 1. [F1] Divergent Change — `cmd/fo/main.go`

- **Diagnosis:** Divergent Change (single-axis-of-change).
- **Why:** 79 commits in 6mo touch this file under at least 15 distinct verb-cluster prefixes (`feat(cli)` 9, `feat:` 7, `fix:` 6, `refactor(fo)` 5, `feat(wrap)` 4, `fix(stream)` 3, `simplify` 2, `refactor(render)` 2, `refactor(cmd/fo)` 2, `fix(state)` 2, `fix(lint)` 2, `feat(dashboard)` 2, plus singletons in `refactor(wrapper)`, `refactor(stream)`, `refactor(editor)`). Rule threshold is ≥10 commits + ≥4 verb clusters; this file is well past both. Commit subjects show it absorbs CLI flags, wrap dispatch, stream/buffered routing, stdin sniffing, state side-effects, schema printing, and error-message tuning.
- **Evidence:** `git log --since=6.months.ago --oneline -- cmd/fo/main.go | wc -l` → 79. Subjects sampled include `feat(watch): fo watch -- <cmd> subcommand`, `feat(wrap): add archlint-text wrapper`, `feat(cli): auto-route fo:metrics`, `fix(stream): coalesce intermediate snapshots`, `fix(state): surface state.Save failure`. File path: `/Users/vcto/Projects/fo/cmd/fo/main.go`.
- **Fix:** Split `main.go` along its accreted axes. Likely seams: a `cliflags.go` (flag wiring + `--as`/`--print-schema`/`--version`), a `dispatch.go` (subcommand + wrap routing already partially hinted by the architecture map), a `sniff.go` (the `sniffSARIF`/`sniffGoTestJSON` cluster), leaving `main.go` as the thin orchestrator. Each new file's commit history should then narrow to ≤2 verb clusters.

## F2

### 2. [F2] Divergent Change — `pkg/testjson/parser.go`

- **Diagnosis:** Divergent Change.
- **Why:** 19 commits in 6mo across 7 verb clusters — `fix` (8), `refactor` (4), `feat` (3), plus topic-prefixed commits (`fo-u2w`, `fo-op6`, `fo-gn0, fo-18j`, `docs`). The parser is doing simultaneous duty as: streaming reader, tolerant fallback parser, error-mapping layer, and aggregator. Sampled subjects: `fo-op6: stream stdin incrementally`, `fo-fnw: parseToReport falls back to tolerant testjson parser`, `fix(stream): coalesce intermediate snapshots`, `fix(parse): preserve real parse errors`.
- **Evidence:** `git log --since=6.months.ago --oneline -- pkg/testjson/parser.go | wc -l` → 19. File: `/Users/vcto/Projects/fo/pkg/testjson/parser.go`.
- **Fix:** Carve the streaming path (`ParseStream`/coalescer) out from the buffered `ParseBytes` path and from the tolerant-fallback recognizer. The aggregator (`*aggregator`, seen 6× as receiver) is its own concept and deserves its own file. After the split, each file's commit verbs should collapse toward a single axis.

## F3

### 3. [F3] Shotgun Surgery — wrapper trio (archlint × jscpd × diag)

- **Diagnosis:** Shotgun Surgery across `pkg/wrapper/wrap{archlint,jscpd,diag}`.
- **Why:** Three pairs co-change at the threshold across three sibling packages: `wraparchlint/archlint.go ↔ wrapjscpd/jscpd.go` (8 co-changes), `wrapdiag/diag.go ↔ wrapjscpd/jscpd.go` (6), `wraparchlint/archlint.go ↔ wrapdiag/diag.go` (5). The rule fires at ≥5 co-changes spanning ≥3 distinct pkgs — all three conditions met. This is the classic "every new wrapper feature requires editing every wrapper" signature: registry plumbing, `Convert`/`RegisterFlags` split, `FixCommand` generators, boundread cap, lint-silence patches all landed as parallel edits.
- **Evidence:** Co-change counts from the standard 6mo shotgun query (see report header). Commit examples on `archlint.go` that touch the other wrappers in the same SHA: `fo-op6: stream stdin incrementally; bound buffered path via boundread`, `fo-ffy: per-wrapper FixCommand generators`, `refactor(wrapper): split Wrap into RegisterFlags+Convert, framework owns orchestration`, `feat(wrapper): add descriptions to registry`. Files: `/Users/vcto/Projects/fo/pkg/wrapper/wraparchlint/archlint.go`, `/Users/vcto/Projects/fo/pkg/wrapper/wrapjscpd/jscpd.go`, `/Users/vcto/Projects/fo/pkg/wrapper/wrapdiag/diag.go`.
- **Fix:** The architecture doc notes wrappers are intentionally a "no interface, no registry" pattern with a `Convert(in, out) error` shape — but the *operational* cross-cutting concerns (FixCommand generation, bound-read enforcement, flag/description plumbing) are clearly being duplicated. Hoist those into a shared `pkg/wrapper/wrapcore` (or extend `pkg/wrapper/registry.go`) so a new wrapper-wide concern is one edit, not N. Specifically: a single helper for "ReadAll bounded by 256 MiB" and a single FixCommand-template hook would eliminate ~half the historic co-changes.

## F4

### 4. [F4] Data Clumps — `(args []string, stdin io.Reader, stdout, stderr io.Writer)`

- **Diagnosis:** Data Clumps.
- **Why:** The same four parameters travel together in ≥5 function signatures across `cmd/fo`. The clump represents "the CLI execution environment" and is currently passed positionally everywhere, inviting argument-order swaps between `stdout` and `stderr` (both `io.Writer`, same static type) that the compiler can't catch.
- **Evidence:** Signature frequency scan (regex over `func ...(...)`): `(args []string, stdin io.Reader, stdout, stderr io.Writer)` appears 5 times in current tree. Closely related: `renderMode(mode string, r *report.Report, stdout io.Writer, themeName string)` at `/Users/vcto/Projects/fo/cmd/fo/main.go:313` is one swap-bug away from the same hazard.
- **Fix:** `type CLIEnv struct { Args []string; Stdin io.Reader; Stdout, Stderr io.Writer }` and pass `*CLIEnv`. Constructed once in `main`, threaded through. Removes 5 four-arg signatures and makes the `stdout/stderr` distinction nominal (and named at the call site).

## F5

### 5. [F5] Primitive Obsession — `mode string` and `themeName string` in render dispatch

- **Diagnosis:** Primitive Obsession (low-grade, single-site).
- **Why:** `renderMode(mode string, r *report.Report, stdout io.Writer, themeName string)` accepts two bare `string`s representing finite enumerated domains (`human|llm|json` for mode; `color|mono` for theme per CLAUDE.md). `pkg/report` already defines `type Severity string` and `type TestOutcome string` for exactly this pattern — the convention exists but isn't applied at the render boundary.
- **Evidence:** `/Users/vcto/Projects/fo/cmd/fo/main.go:313`. Comparator: `/Users/vcto/Projects/fo/pkg/report/report.go:10` (`type Severity string`), `:21` (`type TestOutcome string`). Rule threshold is ≥3 domain concepts — this is just above the "don't flag if used once" lower bound: the mode string also flows through `resolveFormat` and the theme string through `pkg/theme`, so it propagates.
- **Fix:** `type RenderMode string` + constants in `pkg/view`; `type ThemeName string` + constants in `pkg/theme`. Convert at the flag-parse boundary, reject invalid values once, and let the rest of the codebase work in the typed form. Tracks the project's own established convention.

---

## Phases not flagged

- **Feature Envy / Inappropriate Intimacy:** snipe bundle's `call_graph` was empty; live `rg` of `\b\w+\(\)\.\w+\(\)\.\w+\(\)` produced zero hits (no message chains either). No evidence-grade finding available in this run.
- **Message Chains:** none detected.
- **Wrapper duplication beyond F3:** likely overlaps `/review simplify-flow` and `/review arch` territory; deferred per linter guidance.

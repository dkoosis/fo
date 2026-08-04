# conversion-drift — repo review

RUN_ID: 7858b3a8ea1b

## Scope note

`conversion-drift` is diff-scoped by design (its own body: "Don't run this
against full repo"). Dispatched here at `scope: project`, so I applied it to
the one real diff in flight — this branch (`fo-w1f-tee-to-log`, 3 commits
ahead of `main`: `bac1c3b`, `9a0471a`, `28a52d1`, touching `pkg/state/fulllog.go`,
`pkg/state/state.go`, `cmd/fo/state.go`) — then swept the rest of the repo for
custom conversion helpers (`MarshalJSON`/`UnmarshalJSON`/`Scan`/`Value`) to
confirm nothing else qualifies. There are none outside `encoding/json` struct
tags; the branch's atomic-write core is the only boundary-conversion surface
in the repo.

## Findings

None.

The branch adds `SaveFullLog`/`recordFullLog` (tee raw pre-coercion stdin to
`.fo/full.log`) and refactors `writeAtomic`/`writeAtomicBytes` into a single
`writeAtomicTo(path, tmpPattern, write func(io.Writer) error)` core. Checked
against every rule in `conversion-drift.rules.md`:

- **helper-zero-mapping-changed** — `writeAtomic`'s JSON path is wrapped
  unchanged (same `json.NewEncoder(w)` + `SetIndent` + `Encode(v)` closure,
  same `tmp` `io.Writer`); byte-identical output. Not a semantic change,
  just an extraction — matches the linter's own "skip pure noop refactors."
- **helper-precondition-tightened** — no preconditions added; `SaveFullLog`
  accepts nil/empty `data` same as any other byte slice (writes an empty
  file, no special-cased rejection).
- **driver-version-bump-near-serialize** — no `go.mod` changes in this diff.
- **marshal-unmarshal-asymmetric-diff** — n/a; repo has no custom
  `MarshalJSON`/`UnmarshalJSON`/`Scan`/`Value` methods anywhere (confirmed
  via repo-wide grep), so this rule has no applicable surface.
- **omitempty-added-on-meaningful-zero** — no JSON tag changes in this diff.
- **boundary-helper-no-roundtrip-test** — covered. `pkg/state/fulllog_test.go`
  (write, overwrite, mkdir-failure) + `cmd/fo/state_test.go`
  (`TestRecordFullLog_AppendsFullNotice`, `TestRecordFullLog_StateOffSkips`,
  `TestRun_AsFlag_FullLogPreservesRawInput`, `TestRecordFullLog_WriteFailureRecordsNotice`)
  exercise the zero/empty/failure cases for the new helper.

One genuine zero-mapping bug did exist in this exact class — the pre-fix
`SaveFullLog` discarded the resolved path on `ErrDurabilityDegraded` (treating
a durability warning as a hard failure, unlike `attachDiff`'s precedent) — but
it was already caught (CodeRabbit/SonarCloud on PR #288) and fixed within this
same branch, commit `28a52d1`. Current `pkg/state/fulllog.go:21-31` and
`cmd/fo/state.go:58-76` correctly surface path + a durability notice on that
error path. Nothing left to flag.

# v1 golden fixtures (snapshot for v2 cutover parity check)

Captured 2026-04-26 from a freshly built v1 binary at the head of branch
`v2-substrate-pin` (last v1-touching commit: `0c514ac`). Built with:

    go build -o /tmp/fo-v1 ./cmd/fo

Each fixture has an input file and three rendered outputs:

    <name>.input.<ext>     — raw tool output piped to fo
    <name>.human.golden    — NO_COLOR=1 fo --format human
    <name>.llm.golden      — fo --format llm
    <name>.json.golden     — fo --format json

Wrapper fixtures (jscpd, archlint, gofmt) were rendered through
`cat <input> | fo wrap <wrapper> | fo --format <fmt>`. Direct fixtures
(golangci, gotest) were rendered with `cat <input> | fo --format <fmt>`.

`NO_COLOR=1` was set for all renders to keep human goldens free of ANSI escapes.

## Sources

### golangci/  (3 fixtures)
SARIF 2.1.0 from golangci-lint. Two are real captures from this repo's
own `pkg/sarif/testdata/`; the third (mixed) is a hand-crafted small
multi-severity sample copied from `cmd/fo/testdata/demo/sarif-mixed.json`.

- `clean.input.sarif`   — golangci-lint v1.x post-cleanup snapshot
                          (originally golangci-lint-113-post-cleanup.sarif).
                          Despite the name, it has 113 issues — exercises the
                          large-corpus rendering path.
- `issues.input.sarif`  — 121-issue snapshot (originally golangci-lint-121-issues.sarif).
                          Mixed errcheck / gosec / goconst / etc.
- `mixed.input.sarif`   — Small 3-finding sample with errcheck error +
                          revive warning + gosec error.

### gotest/  (3 fixtures)
go test -json streams.

- `clean.input.json`     — All passing, small package set.
- `mixed.input.json`     — Mixed pass/fail across 2 packages.
- `large-pass.input.json`— Real `fo` test run (passing, 2312 events) from
                          `pkg/stream/testdata/gotest-pass.json`. Stresses
                          the streaming/aggregation path.

### jscpd/  (2 fixtures)
jscpd `--reporters json` output.

- `duplicates.input.json` — Real jscpd run on a small synthetic Go pair
                           with one detected duplicate clone (min-tokens=10).
- `clean.input.json`     — Hand-crafted minimal "no duplicates" report.
                           jscpd refuses to write a JSON report when no
                           duplicates exist, so we synthesized the empty
                           shape `{"duplicates":[],"statistics":{...}}`.

### archlint/  (2 fixtures)
go-arch-lint `--json` (i.e. `models.Check` envelope) output.

- `clean.input.json`      — Synthetic "no violations" payload (matches the
                           shape used by `pkg/wrapper/wraparchlint` tests).
                           A real go-arch-lint check on a violating sample
                           project did not produce `ArchWarningsDeps` entries
                           under the v3 schema we tried; the wrapper's own
                           tests use synthesized JSON for the same reason.
- `violations.input.json` — Synthetic 3-violation payload (search→embedder,
                           store→svc, agentSupervisor→shell with full import
                           path) covering both short-name and full-path cases
                           handled by the wrapper.

### gofmt/  (1 fixture)
gofmt `-l` line output (file paths only, one per line) — the input shape
that `fo wrap diag --tool gofmt --rule needs-formatting` consumes.

- `needs-format.input.txt` — Two file paths from a synthetic sample run
                            through `gofmt -l`.

## Known v1 quirks captured (do NOT "fix" in v2 unless intended)

1. **Empty human output on clean wrapper inputs.** When jscpd or archlint
   wrapper inputs report zero findings, `fo --format human` produces an
   empty file. The llm and json formats still emit a header / empty
   patterns block. v2 should explicitly decide whether silence-on-clean
   is the desired UX for these wrappers (gotest clean does emit human
   output; SARIF clean does not — currently format-dependent).

2. **Nondeterministic `generated_at` timestamp.** llm and json goldens
   embed an RFC3339 timestamp at render time. Diffs against these
   goldens must either:
     a) regenerate goldens at compare time and diff structure only, or
     b) substitute a stable value before diffing
        (e.g. `sed -E 's/generated_at: [^ ]+/generated_at: TS/'`).
   The `data_hash` field appears stable across runs (input-derived).

3. **`fo:llm:v1` header.** The llm format prefix is versioned. v2 will
   need to either preserve `fo:llm:v1` (claiming byte-equivalence) or
   bump to `fo:llm:v2` (claiming explicit format change) — the cutover
   review needs to make this an intentional decision.

4. **Stderr merged into goldens.** Captures use `> file 2>&1`. If any
   fixture is expected to emit a stderr warning, it will appear in the
   golden. None observed in this batch.

## How to use these for v2 cutover (fo-7f5.9)

For each fixture:

    cat <name>.input.<ext> [| fo wrap <wrapper>] | fo-v2 --format <fmt>
        | diff - <name>.<fmt>.golden

Expected outcomes per format:
- **byte-equivalent** → no action; record parity in cutover commit.
- **structurally-equivalent + reviewed delta** → document the delta in
  this CHANGES.md (append a `## Deltas (v2)` section with a per-fixture
  rationale) and check in the new v2 goldens alongside under
  `testdata/golden/v2/`.
- **regression** → block cutover; file an issue against the responsible
  v2 substrate package.

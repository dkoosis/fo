# /review focused — diff (last 3h) — 2026-05-16

Run id: 482cfd360229
Scope: 21 files changed since `4a637d7` (the 6-PR drain + earlier same-session work).
Linters: 8 of 25 (concurrency-safety, goroutine-lifecycle, alloc-bounds, errors-design, conversion-drift, zero-sentinel, slice-map, solid).
Excluded: api-surface, change-smells, ctx-value, domain-vocab, io-parallel, json-shape, n-plus-one, pointer-value, sqlite, test-effectiveness, test-tables, truthful-names, tx-boundary, vestige-pair, arch — the diff didn't materially exercise their lenses.

## Scorecard

| Linter              | Verdict | Top finding (one-line)                                                                  | Report |
|---------------------|---------|------------------------------------------------------------------------------------------|--------|
| concurrency-safety  | minor   | watchkey.go:53 close-on-cancel goroutine closes TTY fd before Restore (adj. to fo-oq9)   | [link](review/concurrency-safety-diff-482cfd360229.md) |
| goroutine-lifecycle | green   | main.go:902 `snapshots` chan buffer=8 is uncommented magic number                        | [link](review/goroutine-lifecycle-diff-482cfd360229.md) |
| alloc-bounds        | yellow  | cluster.go:106-151 `RunWith` allocates N slices from `len(inputs)` (no cap)              | [link](review/alloc-bounds-diff-482cfd360229.md) |
| errors-design       | yellow  | scene.go:109 + suppress.go:96 — 5 exported `Err*` sentinels each, zero `errors.Is` callers | [link](review/errors-design-diff-482cfd360229.md) |
| conversion-drift    | yellow  | suppress.go:52 `Expired` instant→day-trunc; `until=YYYY-MM-DD` now inclusive (no version gate) | [link](review/conversion-drift-diff-482cfd360229.md) |
| zero-sentinel       | yellow  | cluster.go:107 dedupe on Input.Key with no empty-key guard (recurrence of fo-juf shape)  | [link](review/zero-sentinel-diff-482cfd360229.md) |
| slice-map           | green   | filter.go:29 in-place filter pins suppressed `Finding` strings in tail backing under watch | [link](review/slice-map-diff-482cfd360229.md) |
| solid               | yellow  | scene.go vs suppress.go duplicate kv-tokenizer (~120 LOC); extract `internal/kvtok`      | [link](review/solid-diff-482cfd360229.md) |

## Cross-linter hotspots

_Locations cited by 2+ linters from different angles. Highest-leverage fixes._

| # linters | location                          | linter:finding:rule                                                                                     |
|-----------|-----------------------------------|---------------------------------------------------------------------------------------------------------|
| 2         | `cmd/fo/watchkey.go:53`           | concurrency-safety:F1:race, goroutine-lifecycle:F2:no-owner — fd/restore ordering AND join discipline   |
| 2         | `pkg/cluster/cluster.go:106-151`  | alloc-bounds:F1:unbounded, zero-sentinel:F1:empty-key-dedupe — `RunWith` is unbounded and loses inputs  |
| 2         | `pkg/suppress/suppress.go` (sentinels)       | errors-design:F2:sentinel-no-callers, solid:F1:duplicated-kv-tokenizer                       |
| 2         | `pkg/suppress/suppress.go:52,163` | conversion-drift:F1:expired-semantics, zero-sentinel:F3:zero-year-accepted — both about `Expired`       |
| 2         | `pkg/scene/scene.go` (kvtok)      | errors-design:F1:sentinel-no-callers, solid:F1:duplicated-kv-tokenizer                                  |

## Already filed (avoid double-filing)

- fo-oq9 — watchkey restore race (sync.Once). concurrency-safety F1 is *adjacent*, not a duplicate.
- fo-juf — cluster Key collapses retried failures. zero-sentinel F1 is the same family, broader scope.
- #257 — `outputBuf` unbounded byte growth. alloc-bounds F2 finds the orthogonal *key-count* axis.

## Suggested new beads

1. **suppress: `Expired` semantics shift + zero-year rule** — conversion-drift F1 + zero-sentinel F3. Add day-boundary test, reject zero-year at Parse, add `Format/Parse` roundtrip test for escapes. (P2)
2. **cluster: cap `RunWith` inputs + empty-Key guard** — alloc-bounds F1 + zero-sentinel F1. Relates to fo-juf. (P2)
3. **kvtok: extract shared tokenizer for scene + suppress** — solid F1, before they drift on escape semantics. (P3)
4. **filter: clear suppressed tail in `ApplyFilter`** — slice-map F1. Material under `fo watch` reruns. (P3)
5. **errors-design: unexport scene + suppress `Err*` sentinels** — errors-design F1+F2. 10 sentinels, zero callers. (P3)

## Next

→ `/assess-feedback` per linter, or file beads 1-5 above directly.

Caveat: two target files in the linter prompts didn't exist (`pkg/testjson/stats.go`, `pkg/wrapper/wraparchlint/convert.go`); reviewers scanned the 18 that did. Doesn't affect findings on real files.

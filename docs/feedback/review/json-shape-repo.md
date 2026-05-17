# json-shape — repo review

run-id: bd775e303d86-json-shape
target: repo
date: 2026-05-17

## Summary

Three lossy-omitempty findings — fields where the JSON-absent and JSON-zero forms
collapse into the same struct value, but the domain treats them as distinct. No
float-money fields, no public-handler decoders (CLI only, decoders consume
trusted upstream tool output), no `any`-decode targets in production code, no
custom MarshalJSON/UnmarshalJSON to roundtrip-test. Time formats are uniform
(`time.Time` default RFC3339, `time.Duration` int ns).

Tier rollup: P1 omitempty 3 → 🔴 by count; severity moderated by all three being
internal IR, not external wire — readers of `fo`'s JSON output are LLMs and
downstream tooling, where field-absent vs zero ambiguity still bites.

---

### 1. [F1] `pkg/report/report.go:97` — omitempty-loses-meaningful-zero

**Diagnosis:** `Suppressed int json:"suppressed,omitempty"` on the canonical
`Report` IR drops the field when zero. Zero is the meaningful "no suppressions
matched" state, distinct from "field absent because writer didn't compute it".

**Why it matters:** The doc-comment two lines above the field literally says
"Zero when no suppressions matched or no .fo/ignore file was loaded" — that is
the exact case the wire format then hides. A JSON consumer (LLM, downstream
tool) cannot distinguish "fo ran ignore-matching and got 0 hits" from "this
Report predates the field". CI dashboards counting suppression drift will
silently treat both as the same.

**Evidence (Read-verified):**
```go
// pkg/report/report.go:94-97
// Suppressed counts findings removed by .fo/ignore active rules
// during this run. Zero when no suppressions matched or no .fo/ignore
// file was loaded.
Suppressed int `json:"suppressed,omitempty"`
```

**Fix:** Drop `,omitempty`. The field is always meaningful — a fresh run that
matched nothing should emit `"suppressed":0`, not drop the key. Cost: one extra
byte per Report. Benefit: consumers can rely on the field's presence as a
schema-version signal.

**Tier:** 🟡 P1 lossy-omitempty.

---

### 2. [F2] `pkg/report/report.go:43` and `pkg/report/report.go:57` — omitempty-loses-meaningful-zero

**Diagnosis:** `Finding.Score float64 omitempty` and `TestResult.Score float64
omitempty` drop the score when it computes to exactly 0.0. Score is the
severity-weighted priority emitted by `pkg/score`; 0.0 is a valid scored value
("lowest priority, but ranked"), not a sentinel for "not yet scored".

**Why it matters:** Two distinct domain states collapse to the same wire form —
"scorer ran and produced 0" vs "scorer did not run". Downstream sort/filter
("show me top-N by score") sees the absent field as `0` after unmarshal, which
is correct only by accident. If `pkg/score` ever changes its "no score" sentinel
from 0 to `math.NaN` or a `*float64`, the JSON contract silently shifts.

**Evidence (Read-verified):**
```go
// pkg/report/report.go:34-44
type Finding struct {
    ...
    Score       float64  `json:"score,omitempty"`
}
// pkg/report/report.go:49-59
type TestResult struct {
    ...
    Score       float64       `json:"score,omitempty"`
}
```

**Fix:** Two options, pick one and apply to both:
- (a) Drop `,omitempty`. Always emit the score. Simpler; matches "score is
  always computed" reality.
- (b) Change to `*float64`. Nil = "not scored", non-nil = explicit value. Pay
  the pointer cost only if "not scored" is a real state the IR needs to
  represent.

Today's behavior matches neither — it implicitly treats 0 as "unscored", which
the code does not promise.

**Tier:** 🟡 P1 lossy-omitempty.

---

### 3. [F3] `pkg/state/metrics_history.go:33` — omitempty-loses-meaningful-zero

**Diagnosis:** `MetricDelta.New bool json:"new,omitempty"` drops the field when
false. False has explicit meaning here: "this sample matched a prior; Delta is
real". True means "no prior; Delta and Prior are zeros that should be ignored".
omitempty collapses the false branch into "field absent".

**Why it matters:** A consumer reading the JSON to render delta sparklines must
know `New` to decide whether to draw the arrow. With the current tag, `{}`
unmarshals to `New:false` — same as a deliberately-false delta. Today's renderer
(`pkg/state/diff.go` callers) hits this only because it reads the in-memory
struct, not the JSON; the JSON contract is broken for any external consumer.

**Evidence (Read-verified):**
```go
// pkg/state/metrics_history.go:29-34
type MetricDelta struct {
    Sample MetricSample `json:"sample"`
    Prior  float64      `json:"prior"`
    Delta  float64      `json:"delta"`
    New    bool         `json:"new,omitempty"` // no prior sample matched
}
```

**Fix:** Drop `,omitempty`. Bool flags whose false value carries information
should always emit. Same one-byte-per-row cost; metrics history rows are bounded
by `MaxMetricsHistory = 30`.

**Tier:** 🟡 P1 lossy-omitempty.

---

## Negative findings (verified clean)

- **float-money:** All `float64`-with-`json`-tag fields are sensor-class
  (scores, coverage %, benchmark values, durations as `Elapsed float64`, tally
  counts). No `price`/`amount`/`balance`/`fee`. No flag.
- **decoder-allows-unknown-fields:** All `json.NewDecoder` / `json.Unmarshal`
  call sites consume trusted internal sources: SARIF from linters, `go test
  -json` from `go test`, jscpd JSON, archlint JSON, sidecar state files,
  metrics history files, test fixtures. No HTTP handlers. Per linter's
  don't-flag clause for internal/trusted decoders.
- **decode-target-any:** No `var dst any; json.Unmarshal(b, &dst)` in
  production code. The `mustJSON(t *testing.T, v any) string` helper in
  `pkg/cluster/cluster_property_test.go:110` is a test-only marshal helper, not
  a decode target.
- **marshal-without-roundtrip-test:** Repo defines zero custom `MarshalJSON` /
  `UnmarshalJSON` methods. Nothing to test.
- **time-format-drift:** All times serialize via `time.Time` default (RFC3339)
  or `time.Duration` (int nanoseconds via `json:"duration_ns"`). No drift.
- **json-number-for-int-id / bool-as-string-tag-misuse:** No IDs >2^31, no
  `,string` tags. None.

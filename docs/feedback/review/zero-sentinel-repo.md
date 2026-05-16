# zero-sentinel — repo

Run: f62c7fc3af14 · 2026-05-16 · scope=project · mode=report

Scanned: all `time.Time` fields and uses (no `uuid.UUID` in tree, no custom
`MarshalJSON`/`Scan`/`Value`, no map-index chained reads on non-counter maps).
Surface is small; the live hazards cluster on the `GeneratedAt` field across
three sibling structs that all flow into JSON / sidecar output.

## Summary

| Rule | Findings | Tier |
|---|---|---|
| time-zero-as-missing | 3 | 🟡 |
| optional-value-without-pointer | 1 | 🟢 |
| map-index-no-comma-ok | 0 | 🟢 |
| nil-uuid-as-sentinel | 0 | 🟢 (no uuid dep) |
| string-empty-as-absent | 0 | 🟢 |
| boundary-zero-roundtrip | 0 | 🟢 (no custom marshal/scan) |
| nil-vs-empty-slice-conflated | 0 | 🟢 |

Overall: 🟡 — one persistence-path hazard (F1) reachable through a normal
control-flow branch, two adjacent risks that depend on caller discipline.

---

### 1. [F1] merged Report.GeneratedAt left zero on all-failed-section parse

- **Site:** `cmd/fo/main.go:754`
- **Zero value:** `time.Time{}`
- **Domain meaning conflated:** "report not stamped" vs "report generated at year 0001"
- **Failure mode:** `merged := &report.Report{Tool: "multi"}` starts with a
  zero `GeneratedAt`. The loop only advances it when a sub-report parses
  successfully (`sub.GeneratedAt.After(merged.GeneratedAt)`). If every
  section fails (`perr != nil`, `continue`) or every body is empty (the
  `len(body) == 0` skip), the merged Report exits with `GeneratedAt =
  time.Time{}` and that zero is JSON-encoded as `"generated_at":
  "0001-01-01T00:00:00Z"` — the exact pgx-style sentinel the rule warns
  about. Downstream consumers (state sidecar, metrics history, any tail
  reader) cannot distinguish "this run produced no parseable section"
  from "this run was generated at the dawn of the Gregorian calendar".
- **Code:**
  ```go
  merged := &report.Report{Tool: "multi"}
  for _, sec := range sections {
      ...
      sub, perr := parseSection(sec, body, stderr)
      if perr != nil { ...; continue }
      ...
      if sub.GeneratedAt.After(merged.GeneratedAt) {
          merged.GeneratedAt = sub.GeneratedAt
      }
  }
  return merged, nil
  ```
- **Fix:** stamp `merged.GeneratedAt = time.Now().UTC()` at construction,
  matching `pkg/testjson/toreport.go:25` and `pkg/sarif/toreport.go:23`.
  Then the `After` step only narrows when sub-reports carry a later
  time — and the field is never zero on the wire.

---

### 2. [F2] state.Run.GeneratedAt is `time.Time` (value) — zero-on-write is masked, not prevented

- **Site:** `pkg/state/state.go:67` (struct), `pkg/state/state.go:240-245` (guard)
- **Zero value:** `time.Time{}`
- **Domain meaning conflated:** "run never stamped" vs "epoch run"
- **Failure mode:** `NewRun` does the right thing —
  `if ts.IsZero() { ts = time.Now().UTC() }` — but the *struct* still
  exposes a value `time.Time`. Any future caller that constructs a
  `state.Run{Findings: ..., Tests: ...}` literal without going through
  `NewRun` writes `0001-01-01...` to the sidecar JSON. The guard is at
  the wrong layer: it lives in a constructor, not the type. The pgx
  incident was exactly this shape — guards in app code, zero permitted
  in the type, driver swap silently writes the sentinel.
- **Code:**
  ```go
  type Run struct {
      GeneratedAt time.Time           `json:"generated_at"`
      ...
  }

  // NewRun
  ts := r.GeneratedAt
  if ts.IsZero() {
      ts = time.Now().UTC()
  }
  return Run{ GeneratedAt: ts, ... }
  ```
- **Fix:** either (a) make the field `*time.Time` with `omitempty` and
  treat nil as "not stamped, defaulted on read", or (b) keep the value
  but add an `IsZero` guard inside `Save` (the only write seam) so the
  invariant lives with the persistence boundary. Option (b) is the
  minimum change.

---

### 3. [F3] state.MetricsRun.GeneratedAt has no IsZero guard at the write seam

- **Site:** `pkg/state/metrics_history.go:37-40`, `pkg/state/metrics_history.go:73`, `pkg/state/metrics_history.go:100`
- **Zero value:** `time.Time{}`
- **Domain meaning conflated:** "sample set not stamped" vs "epoch sample set"
- **Failure mode:** Current call sites at lines 73 and 100 both set
  `GeneratedAt: time.Now().UTC()`, so today the field is always
  populated. There is no guard in `Save`-equivalent code (the package
  marshals the envelope directly). If a future code path appends to
  `hist.Runs` without setting `GeneratedAt` — symmetrical to F2 — the
  persisted history will contain a year-0001 sample group. Trend windows
  and sparklines that order by `GeneratedAt` will silently rank that run
  as the oldest forever, corrupting trend math without any error.
- **Code:**
  ```go
  type MetricsRun struct {
      GeneratedAt time.Time      `json:"generated_at"`
      Samples     []MetricSample `json:"samples"`
  }
  ...
  Runs: []MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: legacy}}
  ...
  hist.Runs = append([]MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: samples}}, hist.Runs...)
  ```
- **Fix:** add an IsZero default in the save path (mirroring `NewRun`),
  or wrap the append in a `NewMetricsRun` constructor that stamps the
  field. Type-level fix (`*time.Time`) is overkill here — the field is
  required by contract; a constructor is enough.

---

### 4. [F4] report.Report.GeneratedAt is value-typed for a required-on-wire field

- **Site:** `pkg/report/report.go:67-78`
- **Zero value:** `time.Time{}`
- **Domain meaning conflated:** the field is required by the JSON schema
  (no `omitempty`) but the type permits zero. F1 is the live exploit;
  this is the latent type-level shape that admits it.
- **Failure mode:** Same family as F2 — the constructor discipline lives
  in parsers (`pkg/testjson/toreport.go:25`, `pkg/sarif/toreport.go:23`)
  rather than the type. Every new producer that doesn't import a
  constructor (e.g. the `merged` builder in F1) is one diff away from
  shipping `0001-01-01`. JSON schema consumers reading the schema
  exposed via `fo --print-schema` will see `generated_at: string` and
  trust it.
- **Fix:** lowest-friction is a `report.New(tool string) *Report`
  constructor that stamps `GeneratedAt = time.Now().UTC()`, plus a lint
  rule (or simple grep test) that forbids `report.Report{` literals
  outside that constructor. Pointer typing is unnecessary because the
  domain has no "absent" case for this field — every report has a
  generation time by definition.

---

## Notes / non-findings (kept for audit trail)

- `pkg/testjson/types.go:37` — `TestEvent.Time` is read straight from
  `go test -json` and only round-tripped into intermediate aggregates;
  no path uses it as a domain signal or persists it. Not flagged.
- `cmd/fo/fswatch.go` — `time.Time` channels and timers are all
  in-process control flow, never crossing a wire/persistence boundary.
  Not flagged.
- No `uuid.UUID`, no `sql.Null*`, no custom `MarshalJSON`/`Scan`/`Value`,
  no map-index chained reads on non-counter maps. The repo's surface
  for this linter is genuinely small; almost all risk concentrates on
  the three sibling `GeneratedAt` fields above.

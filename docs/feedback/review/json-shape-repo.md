# json-shape review — fo (repo scope)

run_id: f62c7fc3af14
date: 2026-05-16
linter: json-shape (mode: report, scope: project)

## Summary

fo is a CLI; no HTTP/RPC trust boundary. The wire surfaces are:

- The canonical `Report` JSON contract (pkg/report) emitted on stdout when piped/JSON.
- The sidecars in `.fo/` (`last-run.json`, `metrics-history.json`) that fo owns end-to-end.
- Third-party producer formats fo only reads (SARIF, go test -json, jscpd, archlint, ...). These are forward-compat by design; not flagged.

Score:

| Tier | Result |
|---|---|
| P1 float-money | 🟢 0 (no monetary fields; floats are severity scores / metric values / test durations) |
| P1 lossy omitempty | 🟡 several meaningful-zero fields silently collapse with absent |
| P1 decoder permissive | 🟡 fo's own sidecar loaders don't `DisallowUnknownFields` |
| P2 any decode | 🟢 0 in production code |
| P2 marshal-test | 🟢 no custom `MarshalJSON`/`UnmarshalJSON` defined anywhere |
| P3 time drift | 🟢 single format (`time.Time` default → RFC3339) |

---

## Findings

### 1. [F1] DiffSummary test-outcome deltas use omitempty — absent vs empty collide

- **Field/site:** `pkg/report/report.go:101-103` — `DiffSummary.NewFailures`, `FixedFailures`, `FlakyTests`
- **Issue:** `lossy-omitempty`
- **Failure mode:** Consumers of the JSON contract cannot distinguish "no test-outcome deltas this run" from "this run predates A.3 / no Tests in input". The sibling fields `New`, `Resolved`, `Regressed`, `Flaky` correctly omit `,omitempty` precisely so `[]` is always emitted — these three break that invariant. A downstream dashboard counting `len(report.diff.new_failures)` silently treats both states as zero.

```go
type DiffSummary struct {
    Headline        string     `json:"headline"`
    New             []DiffItem `json:"new"`
    Resolved        []DiffItem `json:"resolved"`
    Regressed       []DiffItem `json:"regressed"`
    Flaky           []DiffItem `json:"flaky"`
    PersistentCount int        `json:"persistent_count"`
    NewFailures     []DiffItem `json:"new_failures,omitempty"`   // ← inconsistent
    FixedFailures   []DiffItem `json:"fixed_failures,omitempty"` // ← inconsistent
    FlakyTests      []DiffItem `json:"flaky_tests,omitempty"`    // ← inconsistent
}
```

- **Fix:** Drop `,omitempty` from these three to match the rest of the envelope. Always emit `[]`. If "feature absent on old runs" needs to be wire-signalled, add an explicit version/capability field instead of leaning on tag presence.

---

### 2. [F2] Finding.Score / TestResult.Score omitempty drops legitimate zero

- **Field/site:** `pkg/report/report.go:43`, `pkg/report/report.go:57`
- **Issue:** `lossy-omitempty`
- **Failure mode:** `Score` is a severity weight produced by `pkg/score`. Score 0 is a meaningful classification (note-tier, no penalty) — distinct from "score not assigned". With `,omitempty` on a `float64`, the JSON consumer cannot tell the two apart, and a renderer keyed on `score` (Leaderboard, sort order) silently degrades to alphabetical / source order for both zero-scored and unscored items.

```go
type Finding struct {
    ...
    Score       float64  `json:"score,omitempty"`
}
type TestResult struct {
    ...
    Score       float64       `json:"score,omitempty"`
}
```

- **Fix:** Drop `,omitempty` (always emit) if 0 is a real score; or change to `*float64` (nil = unscored, 0.0 = "explicitly zero weight"). Whichever you pick, apply the same change to both `Finding.Score` and `TestResult.Score` so the wire shape stays symmetric.

---

### 3. [F3] TestResult.Duration omitempty drops zero-duration tests

- **Field/site:** `pkg/report/report.go:53`
- **Issue:** `lossy-omitempty`
- **Failure mode:** `time.Duration` is an int64; `,omitempty` drops the field for any test whose elapsed time rounded to 0 ns (skipped tests, no-op subtests, fast cached results). Wire consumers can't tell "this test had no measurable duration" from "duration field missing". A latency histogram fed by the JSON contract silently ignores those rows.

```go
Duration    time.Duration `json:"duration_ns,omitempty"`
```

- **Fix:** Drop `,omitempty`. Emit `"duration_ns": 0` explicitly — that's the truthful answer for skipped/cached tests.

---

### 4. [F4] state.Item.Severity omitempty hides a required field on non-resolved classes

- **Field/site:** `pkg/state/diff.go:29`
- **Issue:** `lossy-omitempty`
- **Failure mode:** Every `Item` except `Resolved` carries the current severity, which is the field the wire consumer keys on for rendering. With `,omitempty` and `Severity` declared as a named string type, the field disappears whenever the severity is the empty string — a state that would otherwise be a bug indicator. Consumers reading `item.severity` get `undefined` and silently render the row at the wrong tier (or as a note). The doc comment immediately above the struct says "new/regressed/persistent carry the current snapshot" but the tag undermines that contract.

```go
type Item struct {
    Fingerprint   string          `json:"fingerprint"`
    RuleID        string          `json:"rule_id,omitempty"`
    File          string          `json:"file,omitempty"`
    Severity      Severity        `json:"severity,omitempty"`        // ← required on non-resolved
    PriorSeverity Severity        `json:"prior_severity,omitempty"`  // ← OK to omit
    Class         Class           `json:"class"`
    ...
}
```

The mirrored type `report.DiffItem` (line 86) declares `Severity string `json:"severity"`` correctly — drift between the two shapes.

- **Fix:** Drop `,omitempty` from `Severity` in `state.Item`. Keep `,omitempty` on `PriorSeverity` (genuinely absent for non-regressed classes).

---

### 5. [F5] state.Load does not reject unknown fields in fo's own sidecar

- **Field/site:** `pkg/state/state.go:101-103`
- **Issue:** `decoder-permissive` (own-trust-boundary variant)
- **Failure mode:** `.fo/last-run.json` is fo-owned, single-writer, single-reader. The package's own doc comment ("readers reject unknown versions") promises a strict version gate, but `json.Unmarshal` silently accepts unknown fields. Consequences:
  - A future schema rename (e.g. `Findings` → `findings_by_rule`) reads the old file as version=1, finds no `Findings`, and silently treats every prior finding as resolved — producing a false "all clean" diff on the first run after the rename. Version skew is supposed to be detected via `ErrVersionSkew`; permissive decoding lets a content-shape skew slip past that gate.
  - Hand-edited sidecars with typo'd field names ("findigns") parse successfully and corrupt classification.

```go
func Load(path string) (*File, error) {
    ...
    var f File
    if err := json.Unmarshal(b, &f); err != nil {
        return nil, fmt.Errorf("state: parse %s: %w", path, err)
    }
    if f.Version != SchemaVersion {
        return nil, ErrVersionSkew
    }
    return &f, nil
}
```

- **Fix:** Switch to a decoder and call `DisallowUnknownFields`:

```go
dec := json.NewDecoder(bytes.NewReader(b))
dec.DisallowUnknownFields()
if err := dec.Decode(&f); err != nil { ... }
```

  CLI policy (per the package's existing comment) is to treat parse failure like a missing file — start fresh. So permissive parsing buys nothing; strict parsing converts silent-corruption into a clean re-baseline.

---

### 6. [F6] LoadMetricsHistory permissive decoder masks envelope-vs-legacy ambiguity

- **Field/site:** `pkg/state/metrics_history.go:60-69`
- **Issue:** `decoder-permissive` (own-trust-boundary variant)
- **Failure mode:** The function tries an envelope decode, falls back to a legacy flat-slice decode on either error or `Version <= 0`. Because the envelope decode is permissive, a corrupted/half-written envelope (e.g. `{"version":1,"runs":...}` truncated mid-write) decodes "successfully" with `Version > 0` and an empty `Runs` — masking the corruption and silently losing the entire history. The legacy fallback never fires.

```go
var envelope MetricsFile
if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version > 0 {
    return &envelope, nil
}
// legacy fallback...
```

- **Fix:** Use `json.NewDecoder` with `DisallowUnknownFields` AND check decoder.More() to detect trailing garbage before accepting the envelope. The discriminator between envelope and legacy is then either (a) version present and parse strict, or (b) parse fails → try legacy. This restores the intended branch behavior.

---

### 7. [F7] tally JSON output struct ad-hoc and lossy (informational)

- **Field/site:** `cmd/fo/main.go:349-353`
- **Issue:** Anonymous struct re-declares a wire shape outside `pkg/tally`. `Tool` has `,omitempty`; `Total float64` does not (so `"total":0` always emitted — fine), but `Total` is computed from `[]Row{Value float64}` where the inputs are jscpd duplicate counts / leaderboard counts — i.e. integers carried in a float. Not lossy at current scales, but the wire contract reads as "tally totals are floating-point" which they aren't.

```go
out := struct {
    Tool  string      `json:"tool,omitempty"`
    Total float64     `json:"total"`
    Rows  []tally.Row `json:"rows"`
}{...}
```

- **Fix (optional):** Promote the shape into `pkg/tally` so the wire contract has one definition. Consider `int64` for `Total` (and document `Row.Value` as count-shaped). Skip if you intentionally want floats here.

---

## Non-findings (verified clean)

- **No custom `MarshalJSON` / `UnmarshalJSON`** anywhere in the tree. Nothing to roundtrip-test.
- **No `any` / `interface{}` decode targets** in production code. The only `json.RawMessage` uses are sniff-only probes, which is the correct shape for "decide later".
- **No float-money:** the only `float64` JSON fields are severity score, test elapsed, metric values, and tally totals. None are currency.
- **No time-format drift:** every `time.Time` field uses the default marshaler (RFC3339Nano). No competing custom layouts found.
- **No `,string` tag misuse on bool.**
- **Third-party decoders** (SARIF reader, archlint, jscpd, testjson) intentionally tolerate unknown fields — correct for forward-compat with producer evolution. Not flagged.

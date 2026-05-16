# zero-sentinel review — diff 482cfd360229

**Verdict:** 🟡 yellow. Three latent zero-as-sentinel sites in cluster/report paths. No persistence-path or auth-path violations, so not red — but the cluster `Key` path (the previously-filed fo-juf variant) recurs here and the report.Cluster wire shape can lose singletons silently on round-trip.

Scope: in-memory run-local data, no DB, no JSON null/zero ambiguity at user-facing IO. Severity bounded by that. Findings cap: 6.

---

### 1. [F1] `pkg/cluster/cluster.go:107-114` — map-index-no-comma-ok / string-empty-as-absent

**Zero value:** `""` (empty `Input.Key`).
**Domain meaning conflated:** "fingerprint not computed yet" vs. "this is the one canonical empty-key input".
**Failure mode:** dedupe-by-Key uses `byKey[in.Key] = in` with no guard against `in.Key == ""`. If two or more inputs arrive with empty Key (e.g. fingerprint helper returned `""` because output was empty, or a future caller forgets to populate Key), they silently collapse into a single record with last-write-wins. The surviving member loses the others' Package/Test info from the cluster. Singleton failures without fingerprints — exactly the cases users most want to see — get eaten.

```go
byKey := make(map[string]Input, len(inputs))
for _, in := range inputs {
    byKey[in.Key] = in            // ← no `if in.Key == ""` guard
}
```

Compounding: `attachClusters` at `pkg/testjson/toreport.go:108-114` sets `Key: t.Fingerprint`, and `Fingerprint` can in principle be `""` if the underlying helper degenerates. Then `singleton` branch at cluster.go:210 uses `recs[members[0]].input.Key` as the signature — empty-string signature hashed → all empty-key singletons collapse to the same ClusterID before `disambiguate` ever sees them, but they were already deduped above.

**Fix:** at the top of `RunWith`, drop or synthesize an ID for inputs whose Key is empty:

```go
for i, in := range inputs {
    if in.Key == "" {
        in.Key = fmt.Sprintf("anon-%d", i) // or skip
    }
    byKey[in.Key] = in
}
```

Or document `Input.Key` as required-non-empty and assert at the boundary.

---

### 2. [F2] `pkg/testjson/toreport.go:128-148` — nil-vs-empty / optional-value-without-pointer

**Zero value:** absent `ClusterID` string on `TestResult`.
**Domain meaning conflated:** "test is a singleton (not in any cluster)" vs. "test was never run through the clusterer" vs. "test was in a cluster that got dropped because `len(g.Members) < 2`".
**Failure mode:** the contract documented on `report.Cluster` (report.go:64-65) says singletons "carry no ClusterID and do not appear in Report.Clusters". That's enforced by the `< 2` filter here. But `keyToID` is only populated from groups that pass the filter, so a test that *was* genuinely a singleton group from the clusterer and a test that the clusterer *never saw* (because `isFailureOutcome` was false, or `inputs` was empty) are indistinguishable downstream — both have `ClusterID == ""`. Renderers can't tell "this is a real singleton" from "clusterer never ran".

This is the classic zero-as-three-meanings bug. Today it's harmless because renderers don't need to distinguish; the moment a future renderer wants to badge "clustered-singleton" vs "unclustered" it will silently mis-render.

**Fix:** either a `Clustered bool` companion on `TestResult`, or store the run-local ClusterID even for singletons and gate display on `len(Cluster.Members) >= 2` at render time.

---

### 3. [F3] `pkg/report/filter.go:42-44` — time-zero-as-missing (boundary case)

**Zero value:** `time.Time{}` reachable through `rule.Until` deref.
**Domain meaning conflated:** "no until date" (nil pointer, intended) vs. "until date parsed as zero".
**Failure mode:** `suppress.Suppression.Until` is correctly typed `*time.Time` (suppress.go:41) — pointer-not-value, which is the right move. The deref at filter.go:42 is gated on `rule.Until != nil`. Good. But `Expired` at suppress.go:48-57 compares `today.After(*s.Until)` and if a future codepath ever set `Until = &time.Time{}` (e.g. via JSON unmarshal of `"until": "0001-01-01"` or a programmatic default), `Expired` would return true on essentially every call because `today` is always after year 1. The suppression would silently never apply.

The parser at suppress.go:163-167 uses `time.Parse("2006-01-02", val)` which rejects `"0001-01-01"`? No — it accepts it. So a user could write `until=0001-01-01` in `.fo/ignore` and silently disable the rule with no warning.

**Fix:** in `parseLine` for the `"until"` case, reject `t.IsZero()` explicitly with `ErrInvalidDate`, or document that zero-year dates are forbidden. Same applies to JSON unmarshal if `Ruleset` ever grows a JSON form.

---

### 4. [F4] `pkg/cluster/cluster.go:73-82` — optional-value-without-pointer (config defaults)

`Config` uses `0` and `""` as "use the default". Already documented as the deliberate shipping default ("Zero-value is the shipping default"). This is fine *per se* — fits "Don't flag: legitimate uses where zero IS the domain value." Flagging only to note: if `IDHexLen=0` ever becomes a *valid* user choice ("emit no hex"), the sentinel reads as the default instead. Today no such mode exists. **No fix required**; recorded so a future API extension knows the trap.

---

### 5. [F5] `pkg/scene/scene.go:61-66` — int-zero-as-absent

**Zero value:** `Command.Exit == 0`.
**Domain meaning conflated:** "command succeeded (exit 0)" vs. "no exit trailer was present in the input".
**Failure mode:** `flushCmd` at scene.go:153-158 only sets `Exit` if a trailing `(exit N)` line is present; otherwise Exit stays 0. The LLM renderer at view/scene_llm.go:89 suppresses `(exit 0)` on output. Round-trip is symmetric for the success case. But: if a scene author *intends* to mark a command as "exit unknown / not recorded" vs. "succeeded", they can't. Renderer at view/scene_human.go:98 also suppresses zero. This is fine for the documented contract ("default 0") and matches Unix convention. **Borderline** — not strictly wrong, but a `*int` or `ExitSet bool` would let "no trailer" survive the IR round-trip distinctly from "exit 0 trailer". Mark as design-aware, no change unless a consumer needs the distinction.

---

### 6. [F6] `pkg/sarif/aggregates.go:72-75` — string-empty-as-absent

**Zero value:** `file := "unknown"` substituted when `len(result.Locations) == 0`.
**Domain meaning conflated:** absent location vs. a result genuinely at file path `"unknown"`.
**Failure mode:** any SARIF producer that legitimately emits a result with URI `"unknown"` (unlikely but technically valid) will be merged into the "no-location" bucket. Collisions on the literal `"unknown"` are silent. The companion function `TopFiles` at line 23 just `continue`s on empty Locations — inconsistent treatment of the same condition. **Fix:** sentinel-substitute is fine if documented; better to use a clearly-non-path token like `"<no-location>"` (already used elsewhere for synthetic file values) to make collision impossible, and align TopFiles + GroupByFile on the same convention.

---

## Summary table

| # | Site | Rule | Severity |
|---|------|------|----------|
| 1 | cluster.go:107 | map-index + empty-key dedupe | 🟡 |
| 2 | toreport.go:128 | three-meaning empty ClusterID | 🟡 |
| 3 | filter.go:42 / suppress.go:163 | zero-year until silently disables rule | 🟡 |
| 4 | cluster.go Config | sentinel-default 0/"" | 🟢 (design-aware) |
| 5 | scene.go Command.Exit | int-zero-as-absent | 🟢 (borderline) |
| 6 | aggregates.go:72 | "unknown" string collision | 🟢 |

P1 time: 0. P1 ambiguity: 2 (F1, F2). P1 map-index: 1 non-critical (F1, overlaps). P2 boundary: 1 (F3).
Overall tier: **🟡**.

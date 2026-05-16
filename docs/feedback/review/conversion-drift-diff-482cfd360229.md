# conversion-drift — diff review

Range: `dd49419~10..HEAD` (10 commits, ~1400 LOC).
Scope: type-conversion helpers near boundaries; quiet semantic drift on update.
Date: 2026-05-16.

## Verdict

🟡 — two genuine semantic shifts in `pkg/suppress` (one inclusive-day comparison, one quote-escaping rule) that callers and on-disk `.fo/ignore` files written by older binaries will silently round-trip differently. One marshal/parse asymmetry to verify. No `go.mod` bumps; no DB/JSON wire boundary changes. Most of the diff is *new* code (new files, additive `Report` fields), which is out of scope for conversion-drift — flagged only where pre-existing helpers changed semantics.

## Findings

### 1. [F1] `pkg/suppress/suppress.go:52` — helper-zero-mapping-changed

**Helper:** `Suppression.Expired(now time.Time) bool`

**Old behavior:** `now.After(*s.Until)` — instant-to-instant. A rule with `until=2026-05-16` expired at the first nanosecond past midnight UTC on 2026-05-16 (since `Until` parses to `2026-05-16T00:00:00Z`).

**New behavior:** Day-truncated UTC comparison. `Until` is now **inclusive through the end of the until-day**. A rule with `until=2026-05-16` stays active all day on 2026-05-16 and expires only on 2026-05-17.

**Risk:** Real-world data drift. Any `.fo/ignore` file written under the old binary with `until=YYYY-MM-DD` is silently re-interpreted on the new binary: the rule lives one extra calendar day. Conversely, a freshly-written rule round-tripped through an older binary will appear expired a day early. The change is intentional (matches the new doc comment "Until is inclusive") but it is a wire-format semantic change with no version gate.

**Affected callers:**
- `pkg/report/filter.go:35` — `rule.Expired(now)` in `ApplyFilter` (new code, lives with the change).
- `pkg/suppress/match_test.go`, `pkg/suppress/suppress_test.go` — verify test fixtures don't pin the old behavior at the day boundary.

**Fix:** Add a table-test for the exact day boundary: `until=2026-05-16` evaluated at `2026-05-16T23:59:59Z` (must be active) and at `2026-05-17T00:00:00Z` (must be expired). Verify `now` callers pass UTC or local-time-safe values — `ApplyFilter` is invoked with `time.Now()` (local clock); the helper truncates with `now.UTC().Date()`, which is correct but worth a comment at the call site. Note the semantics change in CHANGELOG/release notes so anyone hand-editing `.fo/ignore` understands the meaning of `until=`.

### 2. [F2] `pkg/suppress/suppress.go:64,84` — marshal-unmarshal-asymmetric-diff

**Helper:** `Suppression.Format() string` (write side) + `writeValue` helper.

**Old behavior:** `Reason` was quoted only when it contained `" \t"` (space/tab/quote). Inner quotes were escaped with `\"`. `Glob` was written raw. No `\\` escaping anywhere.

**New behavior:** Both `Reason` and `Glob` now go through `writeValue`, which quotes when the value contains `" \t\"\\"` (adds backslash to the trigger set) and escapes both `"` → `\"` and `\` → `\\` via `strings.NewReplacer`.

**Risk:** Encode changed; the parser side (`pkg/suppress/suppress.go` parse path, not shown in diff) was not visibly updated in this range. If `Parse` doesn't unescape `\\` → `\`, then `Format(Parse(s)) != s` for any rule whose `reason` or `glob` contains a literal backslash. Equally, a `glob` like `pkg\subdir/**` written by the new encoder will be re-quoted on the next round-trip, then re-escaped, then double-escaped on the run after. (Suppressions are typically hand-written, so this is a low-probability foot-gun, but globs on Windows-style paths or reasons with file paths could trip it.)

**Affected callers:** any code path that loads `.fo/ignore`, calls `Format()` to rewrite it, and saves back. Search for callers of `Format`:
- the diff adds no auto-rewrite path, so this is latent. The hazard activates the moment someone wires `fo suppress edit` or similar.

**Fix:** Verify `Parse` unescapes `\\` and `\"` symmetrically (look in `pkg/suppress/suppress.go` parse loop, not in this diff). Add a roundtrip table-test: `for _, r := range cases { got, _ := Parse(strings.NewReader(r.Format())); assert.Equal(r, got[0]) }` covering reasons containing `\`, `"`, ` `, and globs containing `\` and `*`. If `Parse` does not handle `\\`, either teach it or revert the `\\` escaping in `writeValue` to keep the wire format unchanged.

### 3. [F3] `pkg/cluster/normalize.go:46` — helper-precondition-loosened (low risk)

**Helper:** `extractAnchor(output string, maxLen int) string` — the leading hard-cap on `output`.

**Old behavior:** `output` truncated to `maxLen*8` before scanning. For typical `maxLen=200` → 1600-byte cap.

**New behavior:** Multiplier bumped 8→128. For `maxLen=200` → 25 600-byte cap.

**Risk:** Loosened input acceptance, not a semantic flip. Anchor extraction now scans **16× more input** before bailing. Two follow-on effects worth checking:
- worst-case CPU/alloc in `panicAnchor` / `testifyAnchor` / `firstColonLine` is now 16× higher for adversarial input. The "pathological input" comment suggests this is the intended trade-off (panic headers trail long logs), but the cap relaxation deserves a benchmark or a test with a worst-case input near the new limit.
- output of `extractAnchor` may change for inputs in the 1600–25 600 byte window where the *prior* truncation hid the real anchor. That changes cluster signatures (`pkg/cluster/id.go`), changes cluster membership, and the new `ClusterID` field stamped onto `TestResult` in `pkg/testjson/toreport.go` flows out to renderers and to any external tooling that consumes `Report.Clusters`. Snapshots/golden tests for the LLM renderer (`pkg/view/scene_llm.go`) may need refresh.

**Fix:** Add one benchmark in `pkg/cluster` for `extractAnchor` on a 25KB input. Add one regression case: a panic stack at byte 5000 (previously truncated, now reachable) — assert the extracted anchor is the panic header. Confirm no golden testdata pins the old 1600-byte horizon.

## Out of scope (verified, not flagged)

- `pkg/scene/scene.go` `IsHeader` rewrite to `[]byte` — pure perf refactor, semantics preserved (still trims leading whitespace + blank lines, still prefix-checks). `isOutputLine` widening to also reject tab indent is documented in the diff comment and is a parser-rule change, not a conversion helper.
- `pkg/report/report.go` — additive only (`ClusterID`, `Clusters`, `Suppressed`). All `omitempty` on the new JSON tags; no existing field changed. **Not** `omitempty-added-on-meaningful-zero`.
- `pkg/testjson/toreport.go` `attachClusters` — new code, no replaced helper.
- `pkg/report/filter.go`, `pkg/suppress/match.go`, `cmd/fo/suppress.go`, `cmd/fo/watchkey.go` — entirely new.
- `pkg/view/scene_*.go`, `cmd/fo/watch.go`, `cmd/fo/main.go` — new code or wiring; no boundary-helper semantics changed.
- `go.mod` — no diff in range.

## Coverage gaps to close

| Helper | Test added in this diff? |
|---|---|
| `Suppression.Expired` day boundary | no — add `until=DATE`, eval at `DATE 23:59:59Z` and `DATE+1 00:00:00Z` |
| `Suppression.Format` / `Parse` roundtrip with `\` and `"` | no — add table-test |
| `extractAnchor` at new 128× cap | no — add benchmark + regression case for anchor past old 8× cap |

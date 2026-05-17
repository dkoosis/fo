# conversion-drift — repo review

RUN_ID: bd775e303d86-conversion-drift
Scope: last 30 commits on `main` (no live diff vs origin/main; whole-repo target degraded to recent-history window per linter design).
Diff window: HEAD~30..HEAD, Go files only.
go.mod bumps in window: none → P2 (driver-version-bump-near-serialize) inapplicable.

Findings: 4. Cap 10.

---

### 1. [F1] `pkg/suppress/suppress.go:165-176` — helper-precondition-tightened

Diagnosis: `parseLine` now rejects `until=0001-01-01` (`t.Year() <= 1`) with `errInvalidDate`. Previously this date parsed successfully and became a permanently-expired rule (Expired returns true for all `now`).

Why it matters: any `.fo/ignore` file written by an earlier `fo` version (or any tool/script) that emitted `0001-01-01` as a sentinel "no expiry" will now fail to load the whole file at that line — Parse returns the first error and aborts, so all suppressions after that line are silently dropped. The asymmetry is the real bug: `Format()` does not validate, so a `Suppression{Until: &time.Time{}}` round-trips through `Format → Parse` and now hard-errors on the second leg.

Evidence (Read-verified):
- `pkg/suppress/suppress.go:165-176` — new `t.Year() <= 1` rejection in Parse.
- `pkg/suppress/suppress.go:71-74` (`Format`) — emits `s.Until.Format("2006-01-02")` with no zero-year guard.
- `pkg/suppress/suppress.go:121-123` (`Parse`) — `return nil, fmt.Errorf(...)` on first error, drops every line after.
- Test coverage: `rg "0001-01-01" pkg/suppress` → no roundtrip / zero-case test added in this diff (boundary-helper-no-roundtrip-test also applies).

Fix:
- Mirror the precondition in `Format`: return `""` or document that callers must not pass zero `Until`. Add a typed constructor `NewSuppression` that validates.
- Add a roundtrip test: build `Suppression{Until: &time.Time{}}`, `Format` it, `Parse` it, assert behavior matches intent (either both reject or both accept).
- Consider degrading from "abort Parse" to "skip+notice" so a single bad line doesn't silently disable every later rule.

Tier: 🔴 (persistence path; silent rule-disable on upgrade; no roundtrip test).

---

### 2. [F2] `pkg/report/filter.go:30-65` — helper-zero-mapping-changed

Diagnosis: `ApplyFilter` was extended with `clear(r.Findings[len(kept):])` to zero the dropped tail (fo-zp0). This is a real fix for `fo watch` rerun pinning, but it changes the post-condition of the helper: before, the dropped tail still held valid `Finding` values addressable via `r.Findings[:cap]`; after, those slots are zero structs.

Why it matters: any caller that reslices `r.Findings` back up to its capacity (a not-uncommon pattern in pooled-buffer code) now reads zero `Finding{}` values where it used to read real data. The IR contract on the slice is "len defines the live window" — but the prior helper quietly allowed access beyond `len`. Tightening this in one place without a documented contract change is the "old caller fed the wrong precondition, new code interprets it differently" shape.

Evidence (Read-verified):
- `pkg/report/filter.go:62-65` — `clear(r.Findings[len(kept):])` added; `r.Findings = kept` truncates.
- `rg "Findings\[:cap" pkg` → no callers found re-slicing past `len` today, so active blast radius is zero. Latent contract trap; flag for doc.

Fix:
- Add a one-line comment on `report.Report.Findings` stating "tail beyond len is undefined; do not reslice past len(Findings)".
- Optional: add a regression test asserting the tail is zeroed (locks behavior so a future refactor can't silently revert).

Tier: 🟡 (no current caller affected; latent trap; doc-only).

---

### 3. [F3] `pkg/testjson/toreport.go:18-21,114-127` — helper-precondition-tightened

Diagnosis: `attachClusters` now caps inputs at `maxClusterInputs = 5000` and silently drops failing tests above the cap. Previously every failure flowed into `cluster.RunWith`.

Why it matters: this is a deliberate memory bound (fo-yax) but it tightens the helper's contract from "all failures clustered" to "first 5000 failures clustered, tail dropped without notice". A consumer reading `r.Tests[i].ClusterID` sees `""` on the tail and cannot distinguish "no cluster assigned" from "dropped due to cap". No `Notice` is appended to surface the drop.

Evidence (Read-verified):
- `pkg/testjson/toreport.go:18-21` — new `maxClusterInputs = 5000` constant + comment.
- `pkg/testjson/toreport.go:124-126` — `if len(inputs) >= maxClusterInputs { break }`.
- `rg "Notices\s*=\s*append" pkg/testjson` → no notice emitted on truncation.
- No test in this diff exercises the cap boundary (boundary-helper-no-roundtrip-test).

Fix:
- Append `r.Notices = append(r.Notices, fmt.Sprintf("clusterer: capped at %d inputs; %d failures unclustered", maxClusterInputs, dropped))` when the cap fires.
- Add a unit test that feeds N+1 failures and asserts the notice + that the first 5000 carry ClusterIDs.

Tier: 🟡 (operational drift; pathological-size only; user-visible signal lost).

---

### 4. [F4] `pkg/scene/scene.go:336-356` + `pkg/suppress/suppress.go:189-201` — marshal-unmarshal-asymmetric-diff (variant: shared-tokenizer extraction)

Diagnosis: `tokenizeAttrs` (scene) and `tokenize` (suppress) both delegate to `internal/kvtok.Tokenize` and re-wrap `kvtok.ErrUnclosedQuote` / `ErrStrayQuote` into local sentinels. The wrapping is asymmetric: scene wraps `ErrStrayQuote` as `errUnknownAttr` ("stray '\"' in header"), suppress wraps it as `errMalformedLine` ("stray '\"' outside key=value"). Both lose the original `kvtok` error from the chain because they return a freshly-built `fmt.Errorf("%w: ...", localSentinel)` without including `err` — `errors.Is(returnedErr, kvtok.ErrStrayQuote)` is now false everywhere outside `kvtok` itself.

Why it matters: error-class identity changed silently across the refactor. Any future caller that asks "is this a stray-quote error?" via `errors.Is(err, kvtok.ErrStrayQuote)` will get a false negative through these two boundaries. The unclosed-quote arm in suppress is worse: it returns the bare `errUnclosedQuote` (no `fmt.Errorf` wrap, no position context, no underlying-error link). The pattern `errors.Is(err, ErrNoHeader)` is established repo idiom (tally_test.go:90, metrics_test.go:50); the kvtok sentinels deserve the same treatment.

Evidence (Read-verified):
- `pkg/scene/scene.go:344-352` — wraps with `%w: ...` against scene's local sentinel; does not include `err` in the chain.
- `pkg/suppress/suppress.go:192-197` — same pattern; `errUnclosedQuote` returned without wrap.
- `internal/kvtok/kvtok.go:17-19` — `ErrUnclosedQuote` is exported.
- `rg "errors.Is.*kvtok\." --type go` → only internal uses; no external caller relies on the chain today (latent regression-detection gap).

Fix:
- In both wrappers: `return nil, fmt.Errorf("%w: %w: stray quote", localSentinel, err)` (Go 1.20+ multi-`%w`) so `errors.Is` walks both the local category and the underlying `kvtok` cause.
- Add a table-test in `pkg/suppress` and `pkg/scene` asserting both `errors.Is(err, errLocal)` AND `errors.Is(err, kvtok.ErrUnclosedQuote)` succeed.

Tier: 🟡 (no current external caller; identity loss is silent; cheap to fix at the wrap site).

---

## Tally

| Rule | Hits |
|---|---|
| helper-zero-mapping-changed | 1 (F2) |
| helper-precondition-tightened | 2 (F1, F3) |
| marshal-unmarshal-asymmetric-diff | 1 (F4) |
| driver-version-bump-near-serialize | 0 |
| omitempty-added-on-meaningful-zero | 0 |
| boundary-helper-no-roundtrip-test | co-flagged on F1, F3 |

Overall tier: 🔴 driven by F1 (persistence path, silent disable on upgrade).

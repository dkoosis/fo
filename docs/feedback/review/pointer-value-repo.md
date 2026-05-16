# pointer-value — repo review

- Run: f62c7fc3af14
- Date: 2026-05-16
- Scope: project (/Users/vcto/Projects/fo)
- Linter: pointer-value (mode: report)

## Summary

Codebase is mostly disciplined on pointer-vs-value. Pointer receivers cluster around legitimate mutation (`sarif.Builder`, `state.File`/`Run` parsers) or std-lib-interface satisfaction (`Result` JSON unmarshaling). The clearest defect is in `pkg/wrapper/wrapdiag`, which uses `*string` for every flag field on an internal struct — confirmed by `gcflags=-m` to force `opts` onto the heap. A few P2/P3 candidates around `Finding` pointer aliasing and `*File` from `Append`. Overall tier: 🟡.

## Findings

### 1. [F1] small-by-pointer / pointer-wrapper-no-nil-use — wrapdiag.diag uses *string for every flag field

- **Site:** `wrapdiag.diag` struct + `wrapdiag.Convert` — `/Users/vcto/Projects/fo/pkg/wrapper/wrapdiag/diag.go:34-41`, `/Users/vcto/Projects/fo/pkg/wrapper/wrapdiag/convert.go:27-33`
- **Issue:** pointer-wrapper-no-nil-use (P1)
- **Size estimate:** Each `*string` is 8 bytes vs 16 for `string`; 4 fields = 32 vs 64 bytes — but the struct itself is heap-allocated either way (`&diag{...}`). The pointers are the cargo-cult.
- **Mutation/nil-use:** The only `nil` check is `if d.toolName == nil` at line 45, which is unreachable on the v2 path (`Convert` always sets `&opts.Tool` etc.) and only existed for the (now-removed) plugin FlagSet path per the `DiagOpts` doc comment. Every other dereference (`*d.toolName`, `*d.ruleID`, `*d.level`, `*d.version`) is unconditional. Nil-as-absence has no semantic role.
- **Escape:** `go build -gcflags='-m' ./pkg/wrapper/wrapdiag/` reports `convert.go:20:40: moved to heap: opts` — the entire `DiagOpts` value escapes because four of its string fields are address-taken into the `diag` struct. With value-string fields, `opts` can stay on the stack.
- **Fix:** Change `diag` fields to `string`. Drop the `if d.toolName == nil` guard (replace with `if d.toolName == ""` if defensive check is still wanted — `toolRequired` already covers it). `convert.go` becomes `diag{toolName: opts.Tool, ruleID: opts.Rule, level: opts.Level, version: opts.Version, stderr: opts.Stderr}`. Net: one fewer heap allocation per Convert call and four fewer `*` dereferences per parsed line.

### 2. [F2] ctor-returns-pointer-no-mutation borderline — state.Append returns *File

- **Site:** `state.Append` — `/Users/vcto/Projects/fo/pkg/state/state.go:196`
- **Issue:** ctor-returns-pointer-no-mutation (P3, borderline)
- **Size estimate:** `File` is 32 bytes (int + slice header). Small.
- **Mutation/nil-use:** Caller is `cmd/fo/state.go` writing the result to disk via `state.Save(path, f)`. `Save` takes `*File`. So the pointer flows directly into a mutation-capable API. Returning `File` would force `Save(path, &out)` at the call site — same allocation, just at the caller.
- **Escape:** Not measured for this site; `Save` already mandates a pointer.
- **Fix:** Low-priority. Keep `*File` for consistency with `Load(path) (*File, error)` and `Save(path, *File)`. Flagged only because the rules call for ctor symmetry; in practice the `*File` is justified by the `Save`/`Load` API shape, which itself is justified by the unmarshal target idiom. **Recommend: no action.**

### 3. [F3] pointer-slice-small-element — indexFindings stores *report.Finding aliases

- **Site:** `state.indexFindings` + `state.classifyFinding` — `/Users/vcto/Projects/fo/pkg/state/diff.go:164-173`, `:176`
- **Issue:** pointer-slice-small-element / small-by-pointer (P2)
- **Size estimate:** `report.Finding` ≈ 6 strings + 2 ints + 1 float64 + 1 Severity string ≈ 128–136 bytes. Above the rules' "small ≤ 64 bytes" line.
- **Mutation/nil-use:** The map values are read-only (`makeItem` reads `f.File`, etc.). The pointer is used purely to alias back into the caller's `[]Finding` slice rather than copy. With ~136B values and N findings the copy cost is real but modest; the pointer aliasing is defensible.
- **Escape:** Not measured.
- **Fix:** Leave as-is. Type is too large to satisfy the P2 "small element" rule; aliasing into the caller-owned slice is intentional. **No action — documenting the call as out-of-scope for this linter.**

### 4. [F4] cargo-cult pointer-receiver — sarif.Result.Line / Col / FixCommand

- **Site:** `sarif.Result.Line/Col/FixCommand` — `/Users/vcto/Projects/fo/pkg/sarif/types.go:93,101,110`
- **Issue:** receiver-mix candidate, but rules.md says receiver-mix is out of scope for this linter (defer to `/review api-surface`). Noted only for the api-surface pass.
- **Fix:** Out of scope. Skip.

### 5. [F5] writeSnapshot's first *bool — legitimate mutation

- **Site:** `view.writeSnapshot` — `/Users/vcto/Projects/fo/pkg/view/stream.go:65`
- **Issue:** Not a defect — `*first` is mutated (`*first = false`) to drive the "blank line between snapshots" behavior. Pointer is the correct choice.
- **Fix:** None.

## Tier Roll-up

| Tier | Count | Result |
|------|-------|--------|
| P1 small-by-ptr | 0 | 🟢 |
| P1 pointer-wrapper unused-nil | 1 (wrapdiag.diag) | 🟡 |
| P2 ptr-slice | 0 (one borderline, out-of-rule by size) | 🟢 |
| P3 ctor | 0 actionable (state.Append justified by Save/Load symmetry) | 🟢 |

Overall: 🟡 — single actionable defect, concentrated in `pkg/wrapper/wrapdiag`. Fix is mechanical (drop four `*` from struct fields + Convert call), confirmed by escape analysis to remove a heap allocation per call.

## Methodology

- Read pointer-value.md + pointer-value.rules.md
- Surveyed pointer receivers (`func (x *T)`), `*T` return values, `*T` parameters, `[]*T` slices, and `*primitive` fields
- Spot-checked struct sizes against the ≤ 64-byte rule
- Confirmed F1 with `go build -gcflags='-m' ./pkg/wrapper/wrapdiag/` (moved-to-heap: opts)
- Excluded: pointer-receiver mutation cases (`sarif.Builder`), JSON unmarshal targets (`*File`), legitimate `*bool` mutation (`writeSnapshot`).

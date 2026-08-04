# test-tables — repo review

Scope: whole repo (project). Reviewed every table-test literal (`tests := []struct` /
`cases := []struct`, plus map-keyed variants) across 24 test files spanning
`pkg/scene`, `pkg/multiplex`, `pkg/view`, `pkg/fingerprint`, `pkg/suppress`,
`pkg/sarif`, `pkg/cluster`, `pkg/score`, `pkg/tally`, `pkg/wrapper/wrapdiag`,
`pkg/hygiene`, `pkg/state`, `pkg/testjson`, `pkg/wrapper/wrapcoverprofile`,
`pkg/paint`, `pkg/metrics`, `pkg/status`, `cmd/fo`, `internal/kvtok`.

Overall: the table-tests in this repo are clean. Every table has ≥3 rows sharing
one shape, no unused struct fields, no one-row tables, `go.mod` is on 1.24 with
no stale `tt := tt` rescoping anywhere, and style is consistent (named-struct-slice
+ `t.Run(tc.name, ...)`) within each package. One borderline case below.

### 1. [F1] `cmd/fo/watch_test.go:32-48` — table-per-case-branching

**Diagnosis:** `TestParseWatchArgs`'s loop body branches on `tt.wantErr`, doing a
structurally different check (assert error, return early) in one arm and a
different check (assert no error, then assert `wantCmd` equality) in the other —
the shape the rule catalog's own canonical "Bad" example describes.

**Why:** The rule (`test-tables.rules.md` → `table-per-case-branching`) flags
tables whose branches "do structurally different assertions" because the cases
don't actually share a shape. Splitting would let each table carry only the
fields its shape needs (error-case rows never populate `wantCmd`; success-case
rows never read `wantErr`).

**Evidence** (`cmd/fo/watch_test.go:32-48`, verbatim):
```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := parseWatchArgs(tt.args)
        if tt.wantErr {
            if err == nil {
                t.Fatalf("parseWatchArgs(%v): want error, got nil (cmd=%v)", tt.args, got)
            }
            return
        }
        if err != nil {
            t.Fatalf("parseWatchArgs(%v): unexpected error %v", tt.args, err)
        }
        if !equalSlice(got, tt.wantCmd) {
            t.Fatalf("parseWatchArgs(%v): got %v, want %v", tt.args, got, tt.wantCmd)
        }
    })
}
```

**Fix:** Split into `TestParseWatchArgs_Errors` (fields: `name`, `args`; 3 rows —
empty, no separator, separator only) and `TestParseWatchArgs_Success` (fields:
`name`, `args`, `wantCmd`; 2 rows — basic, flag before separator). Each table then
carries only the fields its shape uses.

**Tier:** borderline — this is a near-exact match for the rule's textbook example,
but the branch is 3 lines, touches a 5-row table for a small arg-parsing helper,
and the `wantErr`-then-return idiom is common, low-cost Go. Splitting buys
marginal clarity for a two-case function; not worth a bead on its own, but worth
a human's eyes if this file is touched again.

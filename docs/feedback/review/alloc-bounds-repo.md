# alloc-bounds — repo

**Scope:** /Users/vcto/Projects/fo (whole repo)
**Mode:** report (no edits)
**Run:** f62c7fc3af14

## Summary

fo is a stdin-fed CLI, not a server — no HTTP/RPC/queue ingress, no goroutine fanout
sized by external input. Defenses in depth are already in place:

- `internal/boundread.All` caps batched stdin at 256 MiB (`DefaultMax`) and is
  used by the main entry (`cmd/fo/main.go:256`) and every wrapper that buffers.
- The three `bufio.Scanner` sites that consume external input all call
  `Scanner.Buffer(make([]byte, 64*1024), 1024*1024)` — 1 MiB max token
  (`pkg/tally/tally.go:75`, `pkg/status/status.go:62`,
  `pkg/wrapper/wrapleaderboard/wrapleaderboard.go:53`,
  `cmd/fo/main.go:451`).
- The streaming testjson path uses `internal/lineread` (per-line bound) and
  caps per-test/per-package buffered output at 1 MiB
  (`pkg/testjson/parser.go:195`, `appendCapped`).
- Channel buffers are constants (8, 1, 1, 1) at `cmd/fo/main.go:902-905` —
  not derived from input.
- All `make([]T, n)` and `make(map[K]V, n)` use `len()` of slices that are
  themselves bounded upstream (e.g. SARIF doc already capped via boundread,
  test results aggregated under 1-MiB-per-bucket caps).

Two minor gaps remain — both low severity given the CLI threat model.

---

### 1. [F1] `cmd/fo/watch.go:121` — scanner-default-buffer

**Diagnosis:** `stdinTriggers` calls `bufio.NewScanner(r)` on stdin with no
`Scanner.Buffer` override — default 64 KiB token cap.

**Why:** Watch mode treats each newline on stdin as a trigger. A single
producer line longer than 64 KiB causes `sc.Scan()` to return false with
`bufio.ErrTooLong`; the goroutine `close`s `ch` and the watch loop silently
exits. This is a usability/robustness gap rather than memory abuse — but it
violates the scanner-default-buffer rule because the input is external and
unbounded by protocol. Symmetric sites in `tally`, `status`,
`wrapleaderboard`, and `main.go:451` all call `sc.Buffer(...)`; this one is
the outlier.

**Evidence:**
```go
// cmd/fo/watch.go:117
func stdinTriggers(ctx context.Context, r io.Reader) <-chan struct{} {
    ch := make(chan struct{})
    go func() {
        defer close(ch)
        sc := bufio.NewScanner(r)        // line 121 — default 64 KiB buffer
        for sc.Scan() {
            ...
```

**Fix:** Match the rest of the codebase:
```go
sc := bufio.NewScanner(r)
sc.Buffer(make([]byte, 64*1024), 1<<20) // 1 MiB cap
```
Or, since this stream is "one trigger per newline" and the line content is
discarded, switch to `lineread.Read` (already used by testjson) which counts
oversize lines as malformed and continues. Rationale: 1 MiB matches the
project's existing convention; an event-trigger stream has no legitimate
need for any token bytes at all.

**Severity:** Info

---

### 2. [F2] `pkg/sarif/reader.go:16` — readall-without-maxbytes (library API)

**Diagnosis:** `sarif.Read(r io.Reader)` calls `json.NewDecoder(r).Decode(&doc)`
on an unwrapped reader. The decoder will buffer arbitrary bytes for a single
JSON value.

**Why:** Inside fo's CLI, every production caller funnels through
`boundread.All` first (`cmd/fo/main.go:256`) and then `sarif.ReadBytes`, so
the 256 MiB cap is effectively enforced. But `sarif.Read` is an exported
`io.Reader` API — any future caller (in-tree wrapper, external importer of
`pkg/sarif`) that passes an unbounded reader bypasses the project's
documented ingress cap. The rule wants the bound co-located with the read,
not held by convention.

**Evidence:**
```go
// pkg/sarif/reader.go:14
func Read(r io.Reader) (*Document, error) {
    dec := json.NewDecoder(r)            // line 16 — no MaxBytes wrap
    var doc Document
    if err := dec.Decode(&doc); err != nil {
```
Callers verified: `cmd/fo/main.go` reaches sarif only via `parseSARIFTolerant`
on bytes that already passed through `boundread.All`. Only tests call
`sarif.Read` with raw readers.

**Fix:** Wrap r at the API boundary with the same default cap used elsewhere:
```go
func Read(r io.Reader) (*Document, error) {
    lr := &io.LimitedReader{R: r, N: int64(boundread.DefaultMax) + 1}
    dec := json.NewDecoder(lr)
    var doc Document
    if err := dec.Decode(&doc); err != nil { ... }
    if lr.N <= 0 {
        return nil, fmt.Errorf("sarif: %w", boundread.ErrInputTooLarge)
    }
    ...
}
```
Rationale for the cap: matches `boundread.DefaultMax` (256 MiB) — the
project's already-chosen ceiling for "any realistic tool report." Keeps
the bound where the read happens; eliminates the trust-the-caller contract.

**Severity:** Info

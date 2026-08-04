# alloc-bounds — fo (repo)

run: `7858b3a8ea1b`-alloc-bounds
scope: project · mode: report

## Context

fo is a stdin-driven CLI, not a server, so the rule catalog's HTTP/queue-shaped
examples (`readall-without-maxbytes` on a handler, `chan-buffer-from-input`,
`unbounded-fanout`) mostly don't have a literal target here — the trust
boundary is "output of an arbitrary/adversarial build tool piped into fo,"
and fo defends it deliberately: `internal/boundread` caps the batch stdin
read at 256 MiB (`cmd/fo/main.go:292`), `pkg/sarif/reader.go` runs a
token-walk depth guard before `Decode` (`maxNestingDepth = 256`), every
scanner sets an explicit `Buffer(64KB, cap)`, sidecar reads
(`pkg/state/state.go:112`, `snapshot.go:108`, `metrics_history.go:65`,
`runlog.go:97`) all route through `boundread.All(f, sidecarMaxBytes)`, and
`pkg/testjson/toreport.go:129` caps cluster inputs at `maxClusterInputs =
5000`. This is a hardened codebase — a prior alloc-bounds pass (F1: bare
`os.ReadFile` in `state.Load`) has since been fixed by wiring `boundread.All`
into the sidecar loader.

One real gap survived that hardening: the one code path (`--stream`) that
was *built specifically* to accept input larger than the 256 MiB cap still
has an uncapped-cardinality map underneath it.

## Findings

### 1. [F1] `pkg/testjson/parser.go:185` — unbounded-map-alloc

**Diagnosis.** `aggregator.packages` (`map[string]*pkgState`), and each
package's `outputBuf`/`outputBufBytes` maps keyed by test name, grow one
entry per *distinct* package/test name seen on the `go test -json` stream,
with no cap on the number of distinct keys — only on bytes held per key.

**Why.** `runStreamCtx`/`runStreamBatch` (`cmd/fo/stream.go:61,178`) exist
precisely so CI callers can process `go test -json` output **larger than**
the 256 MiB `boundread` cap (the comment at `stream.go:177` names this
explicitly: "Closes fo-frl: piped CI callers can opt into streaming and
bypass the 256 MiB boundread cap"). For that design to be safe, memory has
to stay bounded by *something other than total input size*. It does — but
only per-key: `maxPerTestOutputBytes = 1 << 20` (parser.go:195) bounds bytes
buffered for one test/build/panic bucket, fixed for the exact prior bug
(#257, per the comment at line 190-193: "cumulative buffering was
unbounded"). That fix capped bytes-per-bucket, not bucket **count**.
`a.packages` entries are never deleted (the whole run's package list feeds
the final report), and `outputBuf[testName]` for a *failed* test is kept
until end-of-run too (only `handlePass`/`actionSkip` call `delete(...)` —
`handleFail` at line 315 does not). A `go test -json` producer (buggy
harness, or one under adversarial control in a CI supply chain) emitting
events for an unbounded number of distinct package names, or an unbounded
number of distinct failing test names, grows these maps without limit for
the whole run — in exactly the code path whose entire reason to exist is
"accept more than the byte cap allows."

**Evidence.**

```go
// pkg/testjson/parser.go:185-217
type aggregator struct {
	packages map[string]*pkgState
	order    []string
}

// Per-test/per-package output buffering caps. A panicking or runaway test
// can emit hundreds of MB on a single stream; lineread bounds individual
// lines but cumulative buffering was unbounded (#257). Cap at 1 MiB per
// bucket and append a single sentinel when exceeded.
const (
	maxPerTestOutputBytes = 1 << 20 // 1 MiB
	truncationSentinel    = "fo: output truncated (per-test cap exceeded)"
)

type pkgState struct {
	...
	outputBuf        map[string][]string
	outputBufBytes   map[string]int // bytes accumulated per test name
	...
}
```

```go
// pkg/testjson/parser.go:241-253 — packages never pruned
func (a *aggregator) getOrCreate(name string) *pkgState {
	if pkg, ok := a.packages[name]; ok {
		return pkg
	}
	pkg := &pkgState{
		name:           name,
		outputBuf:      make(map[string][]string),
		outputBufBytes: make(map[string]int),
	}
	a.packages[name] = pkg
	a.order = append(a.order, name)
	return pkg
}
```

```go
// cmd/fo/stream.go:173-178 — the design intent that makes this reachable
// runStreamBatch parses go test -json incrementally (so memory never grows
// with input size) but renders a single batch report in the requested mode.
// ...
// Closes fo-frl: piped CI callers can opt into streaming and bypass the
// 256 MiB boundread cap.
func runStreamBatch(opts streamOpts) int {
```

**Fix.** Apply the same pattern already used for the byte-cap (`#257`,
`appendCapped` + `truncationSentinel`) one level up, at key-cardinality
instead of key-bytes: cap distinct package count and distinct
failed-test-per-package count (e.g. `maxDistinctPackages`,
`maxFailedTestsTracked`, sized like `maxClusterInputs = 5000` is for
cluster inputs), and once the cap is hit, roll further distinct names into
a single "N additional package(s)/test(s) truncated" bucket instead of a
fresh map entry — mirroring the existing sentinel-on-overflow idiom instead
of introducing a new one.

**Tier.** action. Rule: `unbounded-map-alloc`.

---

### 2. [F2] `pkg/sarif/reader.go:28` — json-decode-unbounded-array

**Diagnosis.** `sarif.Read(io.Reader)` still calls `json.NewDecoder(r).Decode(&doc)`
directly, with no `io.LimitReader` and no depth guard of its own — unchanged
since the prior alloc-bounds pass flagged it (`docs/feedback/review/alloc-bounds-repo.md`,
prior F2).

**Why.** Both current production callers are safe: `cmd/fo/parse.go:154`
and `:371` call `sarif.ReadBytes([]byte)`, which runs `checkDepth` (the
iterative token-walk guard) *before* handing the same bytes to `Read` —
so today's exposure is nil. But `Read(io.Reader)` is still the named,
exported entry point, and it inherits whatever bound its caller happens to
supply, silently. A future caller reaching for the natural streaming form
(a new wrapper piping a live `os.Stdin`/`net.Conn` straight in, bypassing
`ReadBytes`) gets no depth guard and no size cap — the exact failure mode
`ReadBytes` was built to close for the byte-slice form.

**Evidence.**

```go
// pkg/sarif/reader.go:27-33
func Read(r io.Reader) (*Document, error) {
	dec := json.NewDecoder(r)
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode sarif: %w", err)
	}
```

```bash
$ rg -n 'sarif\.Read\(|sarif\.ReadBytes\(' cmd pkg | grep -v _test
cmd/fo/parse.go:154:		doc, err := sarif.ReadBytes(input)
cmd/fo/parse.go:371:		doc, err := sarif.ReadBytes(body)
```

**Fix.** Unchanged from the prior pass — still unaddressed, so repeating
rather than dropping it: either wrap `r` in `io.LimitReader(r,
boundread.DefaultMax)` inside `Read` and document the cap in its godoc, or
remove the exported `io.Reader` form entirely and keep only `ReadBytes` as
public API, forcing every caller to bound upstream by construction.

**Tier.** borderline. Rule: `json-decode-unbounded-array`.

---

## Notes (not flagged)

- `pkg/state/state.go:112` (`state.Load`) now routes through
  `boundread.All(f, sidecarMaxBytes)` — the prior pass's F1 (bare
  `os.ReadFile`) is fixed.
- `pkg/cluster/cluster.go` allocations are all sized by `len(recs)`, itself
  capped upstream at `maxClusterInputs = 5000` (`toreport.go:129`).
- All stdin wrapper `Convert` functions (`wrapcover`, `wrapcoverprofile`,
  `wrapgobench`, `wraparchlinttext`, `wrapleaderboard`) read via
  `internal/lineread.Read`, which drains-and-drops any single line over
  16 MiB instead of accumulating it — no per-call unboundedness.
- The four goroutines in the repo (`cmd/fo/watch.go:181,190`,
  `cmd/fo/watchkey.go:58`, `cmd/fo/stream.go:85`) are singleton helpers, not
  input-sized fanouts — no `unbounded-fanout` exposure.
- `cmd/fo/fswatch.go:45` scans `.gitignore` with the default 64 KB Scanner
  buffer — a local repo file, not the untrusted-tool-output boundary this
  linter targets; silent truncation on a pathological line is benign here.

trixi log-skill alloc-bounds findings 2 --run-id "7858b3a8ea1b"

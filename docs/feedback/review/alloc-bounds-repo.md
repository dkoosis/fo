# alloc-bounds — fo (repo)

run: `bd775e303d86-alloc-bounds`
scope: project · mode: report

## Context

fo is a stdin-driven CLI, not a server. The relevant trust boundaries are:

1. **stdin** — output of arbitrary build tools (untrusted in CI, trusted locally). Already capped at 256 MiB by `internal/boundread`.
2. **Sidecar files** under `.fo/` (last-run.json, metrics-history.json, ignore, scene) — written by fo itself but readable/plantable by anything with FS access in the repo root.
3. **No HTTP/RPC/queue surface** — entire `readall-without-maxbytes` / `chan-buffer-from-input` / `unbounded-fanout` rule families don't apply.

The pipeline is well-defended on the dominant axis: `boundread.All` precedes every parser (`cmd/fo/main.go:272`, both wrappers), scanners use explicit `Buffer(64KB, 1MiB)`, per-test output is capped at 1 MiB, and `attachClusters` caps inputs at `maxClusterInputs=5000`. Two borderline gaps remain, both on the sidecar-file axis.

## Findings

### 1. [F1] `pkg/state/state.go:94` — readall-without-maxbytes

**Diagnosis.** `state.Load` reads `.fo/last-run.json` with `os.ReadFile` — no size cap.

**Why.** The sidecar is normally written by fo and stays small (KBs). But the file lives at a well-known relative path inside the repo working directory. Anything with FS access (CI cache restore, a malicious `make` target, a planted artifact) can swap in a multi-GB JSON. `Load` then materializes the whole blob plus the decoded `File` struct before `Version` is checked, OOM-ing fo on next invocation. Defense-in-depth gap: every other entry point (stdin, wrappers) is fronted by `boundread`; this one is bare.

**Evidence.**

```go
// pkg/state/state.go:93
func Load(path string) (*File, error) {
    b, err := os.ReadFile(path)
    ...
    var f File
    if err := json.Unmarshal(b, &f); err != nil {
        return nil, fmt.Errorf("state: parse %s: %w", path, err)
    }
    if f.Version != SchemaVersion {
        return nil, ErrVersionSkew
    }
```

`pkg/state/metrics_history.go:55` (`LoadMetricsHistory`) has the identical shape and the same exposure.

**Fix.** Wrap the file in `boundread.All` with a sidecar-appropriate cap (e.g. 16 MiB — large enough for `MaxHistory` runs of realistic projects, small enough that a planted blob fails fast):

```go
f, err := os.Open(path)
if err != nil { ... }
defer f.Close()
b, err := boundread.All(f, 16<<20)
```

Treat `boundread.ErrInputTooLarge` the same way the existing code treats `ErrVersionSkew` — log + start fresh.

**Tier.** borderline. Rule: `readall-without-maxbytes`.

---

### 2. [F2] `pkg/sarif/reader.go:15` — json-decode-unbounded-array

**Diagnosis.** `sarif.Read(io.Reader)` calls `json.NewDecoder(r).Decode(&doc)` with no reader-side bound and no post-decode length check on `doc.Runs[].Results`.

**Why.** Today, every production caller goes through `cmd/fo/main.go:272` (`boundread.All` → `sarif.ReadBytes`), so the reader is already bounded upstream. That's correct for current call sites but fragile — `sarif.Read(io.Reader)` is the public, named API and the test suite uses it directly. A future caller (a new wrapper, a server-mode experiment, a streaming SARIF probe) that picks the reader form gets no protection. The function silently inherits whatever cap its caller happens to provide.

**Evidence.**

```go
// pkg/sarif/reader.go:15
func Read(r io.Reader) (*Document, error) {
    dec := json.NewDecoder(r)
    var doc Document
    if err := dec.Decode(&doc); err != nil {
        return nil, fmt.Errorf("decode sarif: %w", err)
    }
```

```bash
$ rg -n 'sarif\.Read\(|sarif\.ReadBytes\(' cmd pkg | grep -v _test
cmd/fo/main.go:609:    doc, err := sarif.ReadBytes(input)
cmd/fo/main.go:826:    doc, err := sarif.ReadBytes(body)
```

**Fix.** Two options, pick one and commit:

- Make the contract explicit in code: have `Read` wrap `r` in `io.LimitReader` with `boundread.DefaultMax`, mirroring the bytes-form caller's behavior. Document the cap in the godoc.
- Or delete `Read(io.Reader)` and keep only `ReadBytes([]byte)` as the public surface, so callers are forced to bound upstream. Tests can keep a package-private reader form.

**Tier.** borderline. Rule: `json-decode-unbounded-array`.

---

## Notes (not flagged)

- `pkg/cluster/cluster.go` allocates 7 maps/slices sized by `len(inputs)`, but `pkg/testjson/toreport.go:117` already caps `inputs` at `maxClusterInputs=5000`. Bound is real, named, tested (`cluster_wire_test.go:153`).
- `pkg/testjson/parser.go` scanners use `bufio.NewReaderSize(r, 64*1024)` + per-test 1 MiB cap with explicit truncation sentinel.
- `pkg/suppress/suppress.go:111`, `pkg/scene/scene.go:121`, `cmd/fo/watch.go:194` all set `Scanner.Buffer(64KB, 1MiB)` — no `scanner-default-buffer` exposure.
- All four goroutines in the repo (`cmd/fo/watch.go:181,190`, `cmd/fo/watchkey.go:58`, `cmd/fo/main.go:933`) are singleton helpers, not input-sized fanouts.
- `cmd/fo/fswatch.go:45` reads `.gitignore` with an unconfigured Scanner; the default 64 KB token cap is sized correctly for that format — silent truncation on a pathological line is benign here.

trixi log-skill alloc-bounds findings 2 --run-id "bd775e303d86-alloc-bounds"

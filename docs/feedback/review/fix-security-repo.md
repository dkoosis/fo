# fix-security — repo-wide pass

Run: report-only (mode override; reviewer stage 5a only). RUN_ID 7858b3a8ea1b.
Scope: whole fo repo, one condensed pass. Catalog: G104, G115, G306 (+G301/G302), G404.

Pre-work: `golangci-lint run --no-config --enable-only gosec,errcheck ./...`
(→ `/tmp/lint-fix-security.log`, 24 issues: errcheck 12, gosec 12).

**Triage summary — what did NOT survive the catalog filter:**

- **8 print-family errcheck hits** (`fmt.Fprintln/Fprintf/Fprint` to stderr/stdout in `cmd/fo/explain.go`, `cmd/fo/main.go:201`, `pkg/wrapper/wrapdiag/diag.go:94`) — catalog `g104` explicitly excludes print-family io errors. Dropped.
- **G404 `pkg/cluster/cluster_property_test.go:35`** — `rand.New(rand.NewSource(1))`, seeded reproducible test data. Catalog: "correct, not a bug." Dropped.
- **G304 (3×) + G204 (2×)** — file-inclusion / subprocess. Out of catalog (higher-judgment gosec, human review). Evidence only, not surfaced.
- **G115** — none found.

Findings: 2 action (production dir permissions), 2 borderline (low-signal clusters).

---

### 1. [F1] `cmd/fo/render.go:192` — g306-file-permissions

**Diagnosis** — fo's private state directory `.fo/` is created world-readable/executable (`0o755`).

**Why** — gosec G301: a directory holding fo's run state — findings, test outcomes, metrics history (`.fo/last-run.json`, `.fo/metrics-history.json`) — should be private to its owner, not readable by every user on the box. The state files themselves land at `0600` (created via `os.CreateTemp`), but a `0755` parent lets others list and traverse the directory. Least-privilege for a per-user working dir is `0700`; gosec's own threshold is `≤0750`. `state.Dir()` defaults to `.fo` (redirectable via `FO_STATE_DIR`), so this is always fo's own state, never a public docroot.

**Evidence** — `cmd/fo/render.go:192`; rerun `rg -n "MkdirAll\(state.Dir\(\), 0o755\)" cmd/fo/render.go`. Gosec line:
`cmd/fo/render.go:192:12: G301: Expect directory permissions to be 0750 or less (gosec)`
Verbatim line 192:
```
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
```

**Fix** — downgrade the mode literal to `0o700`. No imports change.

```before
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
```

```after
	if err := os.MkdirAll(state.Dir(), 0o700); err != nil {
```

**Tier** — action

---

### 2. [F2] `cmd/fo/suppress_cmd.go:230` — g306-file-permissions

**Diagnosis** — the parent directory for the `.fo/ignore` suppress-rules file is created world-readable/executable (`0o755`).

**Why** — gosec G301, same class as F1. `writeFile` creates the parent of `.fo/ignore` (or `$FO_IGNORE`) — fo's private suppression config — before an atomic temp+rename write. The directory is fo's own state area, so `0700` is the least-privilege default; gosec accepts `≤0750`. `dir` is `filepath.Dir(path)` where `path` is the ignore file (per the file header comment, `.fo/ignore` or `$FO_IGNORE`), never a designated public directory, so the exclusion for public docroots does not apply.

**Evidence** — `cmd/fo/suppress_cmd.go:230`; rerun `rg -n "os.MkdirAll\(dir, 0o755\)" cmd/fo/suppress_cmd.go`. Gosec line:
`cmd/fo/suppress_cmd.go:230:12: G301: Expect directory permissions to be 0750 or less (gosec)`
Verbatim line 230:
```
	if err := os.MkdirAll(dir, 0o755); err != nil {
```

**Fix** — downgrade the mode literal to `0o700`. No imports change.

```before
	if err := os.MkdirAll(dir, 0o755); err != nil {
```

```after
	if err := os.MkdirAll(dir, 0o700); err != nil {
```

**Tier** — action

---

### 3. [F3] `pkg/state/metrics_history.go:64` — g104-unhandled-error

**Diagnosis** — three read-path `defer f.Close()` cleanup calls discard the error inside error-returning loaders; surfaced as one rollup per catalog anti-flood guidance.

**Why** — errcheck flags `f.Close()` unchecked at three `Load*` functions in `pkg/state`. All three handles come from `os.Open` (read-only), so a `Close` failure carries no lost data and folding it into the return would be wrong — the read already succeeded. Catalog `g104` classes best-effort cleanup as low-signal ("usually fine … surface it as a count"). Borderline because the enclosing funcs *do* return `error` (the per-site trigger fires), yet the identical pattern at `pkg/state/state.go:109` (`Load`) is already tolerated un-flagged, so the codebase norm is to leave read-`Close` bare. Mechanical fix if applied: silence with `_ =` + reason to make the intent explicit and satisfy errcheck.

**Evidence** — the three sites, all `defer f.Close()` on an `os.Open` handle in a `(*T, error)` loader:
- `pkg/state/metrics_history.go:64` (`LoadMetricsHistory`)
- `pkg/state/runlog.go:96` (`LoadRunLog`)
- `pkg/state/snapshot.go:107` (`LoadSnapshot`)

Rerun `rg -n "defer f.Close\(\)" pkg/state/`. Gosec/errcheck line:
`pkg/state/metrics_history.go:64:15: Error return value of ` + "`f.Close`" + ` is not checked (errcheck)`
Verbatim line 64:
```
	defer f.Close()
```

**Fix** — replace each `defer f.Close()` with an explicit best-effort ignore carrying a reason. Apply per file (each BEFORE is unique within its own file; three separate Edits). No imports change.

```before
	defer f.Close()
	data, err := boundread.All(f, sidecarMaxBytes)
```

```after
	defer func() { _ = f.Close() }() // best-effort: read-only handle
	data, err := boundread.All(f, sidecarMaxBytes)
```

For `pkg/state/runlog.go:96` and `pkg/state/snapshot.go:107` the following line is `b, err := boundread.All(f, sidecarMaxBytes)`; use that as the trailing context for a unique BEFORE:

```before
	defer f.Close()
	b, err := boundread.All(f, sidecarMaxBytes)
```

```after
	defer func() { _ = f.Close() }() // best-effort: read-only handle
	b, err := boundread.All(f, sidecarMaxBytes)
```

**Tier** — borderline

---

### 4. [F4] `pkg/view/pipeline_golden_test.go:99` — g306-file-permissions

**Diagnosis** — golden-file writers and one test fixture dir use permissive modes (`0o644` files, `0o755` dir) in `_test.go` only; surfaced as one low-priority cluster.

**Why** — gosec G306/G301 fire on four test-only sites. These write golden fixtures during `go test -update` and create a scratch `vendor` dir — dev-time artifacts, never a shipped runtime surface, so least-privilege on disk is close to irrelevant. Borderline (not dropped) so a human can decide between silencing and downgrading; not `action` because the security value is negligible and touching test perms risks churn without benefit. Catalog does not exclude tests outright, hence surfaced rather than dropped.

**Evidence** — cluster (rerun `rg -n "os.WriteFile.*0o644|os.Mkdir.*0o755" cmd/fo pkg`):
- `pkg/view/pipeline_golden_test.go:99` — `os.WriteFile(goldenPath, got, 0o644)`
- `pkg/view/scene_llm_test.go:128` — `os.WriteFile(goldenPath, buf.Bytes(), 0o644)`
- `pkg/cluster/cluster_test.go:97` — `os.WriteFile(goldenPath, append(gotJSON, '\n'), 0o644)`
- `cmd/fo/fswatch_test.go:140` — `os.Mkdir(vendor, 0o755)`

Gosec line:
`pkg/view/pipeline_golden_test.go:99:16: G306: Expect WriteFile permissions to be 0600 or less (gosec)`
Verbatim line 99:
```
					if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
```

**Fix** — pick one: (a) downgrade golden writes to `0o600` and the fixture dir to `0o750`, or (b) add `//nolint:gosec // test fixture, not a runtime path` at each site. Representative downgrade for the cited line (no imports change):

```before
					if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
```

```after
					if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
```

**Tier** — borderline

---

trixi log-skill fix-security findings 4 --run-id "7858b3a8ea1b"

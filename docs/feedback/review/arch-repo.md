# arch review — fo (project scope)

RUN_ID: 7858b3a8ea1b
Perspective: dependency topology, layering conformance, coupling, cycles, surface bloat.

## Summary

Topology is healthy: **0 import cycles** (`snipe metrics --graph=imports --kind=cycles` → empty SCCs), no coupling danger zone (max non-`cmd` Ce = 5 in `pkg/testjson`; `cmd/fo` Ce=19 is the composition root, exempt), no orphan or lazy packages. The two call-graph SCCs are same-package recursion inside `pkg/view` (render.go / delta.go) — not flaggable.

The findings are all **conformance**: `.go-arch-lint.yml` (dated May 17) has drifted behind two package extractions, so the layering contract no longer governs `pkg/hygiene` and `pkg/multiplex`, and it surfaces one genuine reversed edge (`tally → view`). Plus a borderline surface-size note on `pkg/state`.

---

### 1. [F1] `.go-arch-lint.yml` — layering-violation

**Diagnosis:** Two packages extracted since the config was written — `pkg/hygiene` (shared header/scan helpers) and `pkg/multiplex` (delimiter protocol, formerly `pkg/report/multiplex.go`) — are not declared as components. `go-arch-lint check` emits 6 notices: 2 "not attached to any component" and 4 induced "shouldn't depend on" false alarms from real, intended imports.

**Why:** An unattached file is **ungoverned** — go-arch-lint checks nothing about its imports, so the contract silently stops covering new code. The 4 induced notices (`metrics/status/tally → hygiene`, `cmd → multiplex`) are all legitimate imports of a shared leaf/protocol; they fire only because the target isn't a known component. A guard that emits noise on correct code trains readers to ignore it, and the real `tally → view` violation (F2) hides in the same list.

**Evidence:** `go-arch-lint check` (live):
```
Component metrics shouldn't depend on github.com/dkoosis/fo/pkg/hygiene in .../pkg/metrics/metrics.go:19
Component status shouldn't depend on github.com/dkoosis/fo/pkg/hygiene in .../pkg/status/status.go:22
Component tally shouldn't depend on github.com/dkoosis/fo/pkg/hygiene in .../pkg/tally/tally.go:27
Component cmd shouldn't depend on github.com/dkoosis/fo/pkg/multiplex in .../cmd/fo/parse.go:15
File /pkg/hygiene/hygiene.go not attached to any component in archfile
File /pkg/multiplex/multiplex.go not attached to any component in archfile
total notices: 7
```
`ls pkg/hygiene pkg/multiplex` confirms both packages exist; `pkg/report/multiplex.go` no longer exists (moved out). Importers verified live: `hygiene` ← tally, status, metrics; `multiplex` ← cmd/fo/parse.go.

**Fix:** Add the two components and their allowed edges to `.go-arch-lint.yml`. `hygiene` is a leaf (`{ anyVendorDeps: true }`), and `status`, `metrics`, `tally` gain `hygiene` in their `mayDependOn`. `multiplex` is a leaf consumed by `cmd`; add it as a component and to `cmd.mayDependOn`. After that, 6 of the 7 notices clear, leaving only F2 — which is the point: the guard should show one real violation, not seven mixed signals.

**Tier:** action

---

### 2. [F2] `pkg/tally/tally.go:28` — layering-violation

**Diagnosis:** `pkg/tally`, a hygiene-format **parser**, imports `pkg/view`, the **renderer**, so `Tally.ToLeaderboard()` can build and return a `view.Leaderboard`. This inverts the intended layer direction. The config declares `view mayDependOn tally` (view consumes tally); the code has the arrow pointing the other way.

**Why:** This is the one notice in F1's list that adding a component won't resolve — it's a real reversed edge, and the most blast-radius-relevant finding here. Because the config already permits `view → tally`, the day `pkg/view` imports `pkg/tally` (fully legal under the contract) you get a hard import cycle `view ⇄ tally` that the compiler rejects. Structurally, a format parser owning a render type couples the input shape to the output shape — the north-star's point 1 wants formats to parse *to Report*, with rendering downstream; `ToLeaderboard` making a `view.Leaderboard` bypasses that seam.

**Evidence:** `pkg/tally/tally.go`:
```
28:	"github.com/dkoosis/fo/pkg/view"
```
```
113:// ToLeaderboard builds a view.Leaderboard from t. Rows are emitted in
116:func (t Tally) ToLeaderboard() view.Leaderboard {
117:	rows := make([]view.LbRow, len(t.Rows))
...
123:	return view.Leaderboard{Rows: rows, Total: total}
```
`rg 'fo/pkg/tally' pkg/view/*.go` → no import, so no cycle exists *today*; the trap is latent. `go-arch-lint check` flags it: `Component tally shouldn't depend on github.com/dkoosis/fo/pkg/view`.

**Fix:** Move the `Tally → view.Leaderboard` projection into `pkg/view` (e.g. `view.LeaderboardFromTally(t tally.Tally)`), so the arrow runs renderer→format like the other hygiene formats, and delete `pkg/view` from tally's imports. If the inversion is intentional and `ToLeaderboard` must stay on `Tally`, bless it explicitly by adding `view` to `tally.mayDependOn` in the config *and* dropping `tally` from `view.mayDependOn` to kill the latent cycle — don't leave both directions permitted.

**Tier:** action

---

### 3. [F3] `pkg/state` — pkg-surface-bloat

**Diagnosis:** `pkg/state` exports ~52 symbols spanning six distinct sidecar responsibilities, each with its own `Path` / `Load` / `Save` / `FromReport` quartet: last-run (`state.go`), diff classification (`diff.go`), metrics history (`metrics_history.go`), run log (`runlog.go`), full log (`fulllog.go`), snapshot (`snapshot.go`), plus headline/envelope.

**Why:** A reader looking for "where does the run-log get trimmed" or "where is the metrics-history diff" scans one wide package instead of a named boundary. The surface is over the 41-symbol orange threshold, and the sub-domains (diff vs metrics-history vs snapshot) are only loosely related — they share the `.fo/` directory, not a type. Blast radius is low (`Ca=1`, only `cmd/fo` imports it), which is why this is borderline rather than action.

**Evidence:** `rg` non-test exported count → `state: 52`. Files and their exported roots:
```
diff.go:            type Class, Item, Diff; func Classify
metrics_history.go: type MetricSample, MetricDelta, MetricsRun, MetricsFile; func LoadMetricsHistory, LoadMetrics, AppendMetrics, DiffMetrics
runlog.go:          type RunLogEntry, RunLog; func RunLogPath, RunLogEntryFromReport, LoadRunLog, AppendRunLog, SaveRunLog, (*RunLog).RuleSeries
snapshot.go:        type Snapshot; func SnapshotPath, SnapshotFromReport, (*Snapshot).PriorIDs, (*Snapshot).Lookup, LoadSnapshot, SaveSnapshot
fulllog.go:         func FullLogPath, SaveFullLog
state.go:           type File, Run, Severity; func Dir, Path, MetricsHistoryPath, Load, Save, Reset, Append, RunFromReport
```

**Fix:** Consider splitting the independent sidecars into sub-packages (`pkg/state/runlog`, `pkg/state/metrics`, `pkg/state/snapshot`) that share a tiny `pkg/state` core owning `Dir`/`Path`. Or leave it: the package's single job is "own every `.fo/` sidecar," `Ca=1` keeps the surface from leaking widely, and cohesion (LCOM4) is out of arch scope. A human should pick — hence borderline.

**Tier:** borderline

---

trixi log-skill arch findings 3 --run-id "7858b3a8ea1b"

# Cohesion review — fo repo (whole-repo pass)

RUN_ID: 7858b3a8ea1b · scope: whole repo (condensed) · mode: report

## Verdict: cohesion is strong. Two borderline observations, zero action items.

fo's packages are domain-named (no `utils`/`common`/`core`), terminology is
consistent (every hygiene parser uses the same `Parse`/`IsHeader`/`HeaderPrefix`/
`ErrNoHeader`/`ErrNoRows` surface — that's uniformity, not drift), there are no
versioned shadow twins, and no generic basenames. The high raw LCOM4 numbers
(`view`=14, `sarif`=8, `paint`=4) are libraries of independent same-domain
functions — one idea each (render the IR, parse SARIF, draw primitives) — not
junk drawers. No zone-of-pain flag survives: the only concrete package with many
importers is `pkg/report` (Ca=5, D=0.83), and that is the north-star's ratified
IR-as-pure-data-container (decision #1) — the rule's own carve-out covers it.

Both findings below are `borderline`: valid, low-conviction, surfaced for a human
because fo's cohesion floor is high enough that these are the only two joins worth
a second look.

Note on tooling: the cached `snipe` index (`.snipe/index.db`) misattributes
`pkg/multiplex`'s `ParseSections`/`HasDelimiter`/`IsDelimiter`/`IsDelimiterShape`
to `pkg/report`. Those functions live in `pkg/multiplex/multiplex.go` (verified by
`grep`), correctly separated from the IR. LCOM4/name-token readings for `report`
are inflated by that stale attribution and were discounted.

---

### 1. [F1] `pkg/state` — three parallel, independent persistence subsystems in one package — lcom4-split-candidate

**Diagnosis.** `pkg/state` (LCOM4=9, the second-highest in the repo) is not one
sidecar — it is three near-parallel versioned-file subsystems glued by directory
plus a shared atomic-write helper. Each has its own on-disk path, its own schema
version constant, its own retention cap, its own `File`/`Run` type pair, and its
own `Load`/`Append`/`Save` trio, and none references the others.

| Subsystem | File | Path | Version | Cap | Types |
|---|---|---|---|---|---|
| diff / last-run | `state.go` + `diff.go` | `last-run.json` | `SchemaVersion` | `MaxHistory=3` | `File`, `Run`, `Diff`, `Class` |
| metrics history | `metrics_history.go` | `metrics-history.json` | `MetricsSchemaVersion` | `MaxMetricsHistory=30` | `MetricsFile`, `MetricsRun`, `MetricSample` |
| run log | `runlog.go` | `run-log.json` | `RunLogVersion` | `MaxRunLog=100` | `RunLog`, `RunLogEntry` |

**Why.** The three clusters share only low-level `writeAtomic`/`syncDir`
infrastructure in `state.go` — no domain edges cross between them. That is exactly
what LCOM4=9 is reporting: disjoint symbol clusters co-located by directory. The
code's own comment concedes the separation. Splitting along the boundary
(`state/statediff`, `state/metricshist`, `state/runlog`, keeping the shared atomic
writer in a small `state` base) would let each subsystem's version/cap/type triad
evolve without carrying the other two into every reader's head.

**Evidence.**
`/Users/vcto/Projects/fo/pkg/state/runlog.go:15-17` — the author already names the
three-way independence:
```go
// RunLogVersion is the on-disk version for the replay/trend run log,
// independent of the diff sidecar and findings snapshot.
const RunLogVersion = 1
```
`/Users/vcto/Projects/fo/pkg/state/metrics_history.go:18-22` — a fully parallel,
independent version+cap pair for a different file:
```go
const MaxMetricsHistory = 30

// MetricsSchemaVersion identifies the on-disk envelope format. Bump when
// MetricsFile/MetricsRun shape changes incompatibly.
const MetricsSchemaVersion = 1
```
`/Users/vcto/Projects/fo/pkg/state/runlog.go:22` and `:26` —
```go
const MaxRunLog = 100
```
```go
func RunLogPath() string { return filepath.Join(Dir(), "run-log.json") }
```

**Fix.** Not urgent — the shared `writeAtomic` helper and the single `.fo/`
directory make "one sidecar-persistence package" a defensible framing at fo's
size. Watch it: if a fifth sidecar or a fourth `*SchemaVersion` lands, split along
the table above (`metricshist` and `runlog` are the cleanest to carve out first —
they import nothing from `diff`). Until then, leave it and treat this as the
package to keep an eye on.

**Tier:** borderline

---

### 2. [F2] `pkg/status` exports type `State` while sibling package `pkg/state` exists — homonym read side-by-side — terminology-drift

**Diagnosis.** `pkg/status` names its core type `State` (`StateOK`/`StateFail`/
`StateWarn`/`StateSkip`). A sibling package is literally named `state`. Files that
import both — `cmd/fo/render.go`, `cmd/fo/main.go` — force the reader to hold
`status.State` (a hygiene-row label enum) and the `state.` package (sidecar
persistence) in view at once. Same word, two unrelated concepts, one file.

**Why.** Cross-package vocabulary consistency is the property this protects:
readers waste cycles reconciling a token that means one thing as a package prefix
and another as a type. It is the homonym mirror of synonym drift — the same reader
cost. `status.State` is also mildly redundant (a type inside `status` restating
its package name); `Status` or `Label` would read cleaner and remove the clash.

**Evidence.**
`/Users/vcto/Projects/fo/pkg/status/status.go:27`:
```go
type State string
```
`/Users/vcto/Projects/fo/cmd/fo/render.go:14-15` — both names imported into one file:
```go
	"github.com/dkoosis/fo/pkg/state"
	"github.com/dkoosis/fo/pkg/status"
```

**Fix.** Rename `status.State` → `status.Status` (or `status.Label`), with the
enum members following (`StatusOK`/`StatusFail`/…). Purely mechanical, no behavior
change; removes the `state`-vs-`status.State` reconciliation cost in the two
`cmd/fo` files that co-import them. Low value, low cost — a cosmetic clarity win,
not a defect.

**Tier:** borderline

---

trixi log-skill cohesion findings 2 --run-id "7858b3a8ea1b"

# truthful-names — repo

run: `bd775e303d86-truthful-names` · date: 2026-05-17 · scope: whole repo · mode: report

Overall: 🟢 — names are largely honest. Three small contracts to tighten, all P2.

## Summary

| Tier | Pattern | Count |
|---|---|---|
| 🟢 P1 mismatch | receiver / function-body | 0 |
| 🟢 P1 package | generic basename / drift | 0 |
| 🟡 P2 type/field | terminology-drift | 1 |
| 🟡 P2 function | imprecise-function-name | 1 |
| 🟡 P2 type | imprecise-type-name | 1 |
| 🟢 P2 file/module | — | 0 |
| 🟢 P2 test | — | 0 |

Package basenames (`fingerprint`, `score`, `suppress`, `cluster`, `scene`, `tally`, `status`, `metrics`, `paint`, `theme`, `state`, `report`, `sarif`, `testjson`, `view`) all sit inside the package's exported vocabulary. No `util`/`common`/`helpers`. Module `github.com/dkoosis/fo` — trailing segment is the binary name; honest.

## Findings

### 1. [F1] `pkg/suppress/match.go:7-13` — terminology-drift

- **Symbol:** `suppress.Ruleset`, field `Ruleset.Rules`
- **Pattern:** `terminology-drift`
- **Predicted from name:** A `Ruleset` is a set of rules; `.Rules` returns rules.
- **Actual from body:** The element type is `Suppression`, not `Rule`. The package itself is named `suppress`, the file format is `.fo/ignore`, the parsed type is `Suppression`, and the only entry point is `Parse() []Suppression`. Then a single wrapper introduces a new term — "rule" — that has no other surface in the package.
- **Evidence:**
  ```go
  // pkg/suppress/match.go
  type Ruleset struct {
      Rules []Suppression
  }
  func NewRuleset(rs []Suppression) *Ruleset {
      return &Ruleset{Rules: rs}
  }
  func (rs *Ruleset) Match(ruleID, path string) int { ... }
  ```
  Caller side (`pkg/report/filter.go:ApplyFilter`) takes `*suppress.Ruleset` and iterates suppressions through it. Two readers — the one inside `suppress` (sees `Suppression`) and the one outside (sees `Ruleset.Rules`) — speak different dialects for one concept.
- **Fix:** Rename type and field to match the package's existing vocabulary.
  ```
  suppress.Ruleset                 → suppress.Set            (or suppress.Suppressions)
  Ruleset.Rules                    → Set.Items               (or .List)
  NewRuleset(rs)                   → NewSet(rs)
  *suppress.Ruleset (caller types) → *suppress.Set
  ```
  Grep map:
  ```
  rg -l 'suppress\.Ruleset|NewRuleset' --type go
  # pkg/report/filter.go, pkg/report/filter_test.go, pkg/suppress/match*.go, cmd/fo/main.go (if wired)
  ```
- **Tier:** 🟡 P2

---

### 2. [F2] `pkg/state/metrics_history.go:94` — imprecise-function-name

- **Symbol:** `state.AppendMetrics`
- **Pattern:** `imprecise-function-name`
- **Predicted from name:** "Append" reads as a pure append — open file, write samples at the end.
- **Actual from body:** Load full history → prepend new run → trim to `MaxMetricsHistory` → re-serialize the whole envelope → atomic write. Three concerns the name doesn't carry: (a) prepend, not append; (b) trimming/eviction with a configurable bound; (c) full read-modify-write of the on-disk envelope, not an incremental write. The doc comment even calls out that it replaces "the prior overwrite-only `SaveMetrics`" — but the new name swung past "rotate" or "record" all the way to a verb that means the opposite of what the function does at the slice level.
- **Evidence:**
  ```go
  // pkg/state/metrics_history.go:94
  func AppendMetrics(path string, samples []MetricSample) error {
      hist, err := LoadMetricsHistory(path)
      ...
      hist.Runs = append([]MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: samples}}, hist.Runs...)
      if len(hist.Runs) > MaxMetricsHistory {
          hist.Runs = hist.Runs[:MaxMetricsHistory]
      }
      // ... atomic write
  }
  ```
- **Fix:** Rename to a verb that names the actual operation. Preferred: `RecordMetricsRun` (mirrors `RunFromReport` vocabulary, names the unit being captured, doesn't lie about insertion order).
  ```
  state.AppendMetrics       → state.RecordMetricsRun
  ```
  Alternatives if a shorter name is desired: `state.RotateMetrics` (foregrounds the eviction), `state.PrependMetricsRun` (literal-but-ugly).
  Grep map:
  ```
  rg -l 'state\.AppendMetrics|AppendMetrics\(' --type go
  # pkg/state/metrics_history*.go, plus whichever cmd/wrapper calls it (likely cmd/fo + a wrap* package)
  ```
- **Tier:** 🟡 P2

---

### 3. [F3] `pkg/sarif/aggregates.go:9` — imprecise-type-name

- **Symbol:** `sarif.FileIssue`
- **Pattern:** `imprecise-type-name` (close cousin of `terminology-drift`)
- **Predicted from name:** A `FileIssue` is one issue in a file — the SARIF `Result` projected to its file location. Reader expects `len(TopFiles(doc, 10)) == 10 single issues`.
- **Actual from body:** It's an aggregate row: `{File, IssueCount, ErrorCount, WarnCount}`. There is no single issue here — it's the "per-file rollup" shape used to feed a leaderboard render. `TopFiles` returns up to `limit` *files* (each carrying counts), not up to `limit` issues.
- **Evidence:**
  ```go
  // pkg/sarif/aggregates.go
  type FileIssue struct {
      File       string
      IssueCount int
      ErrorCount int
      WarnCount  int
  }
  func TopFiles(doc *Document, limit int) []FileIssue { ... }
  ```
  Reinforced by the field name: a struct whose own field is `IssueCount` is obviously not "one issue". And `ErrorCount + WarnCount` need not equal `IssueCount` (note-level results count toward `IssueCount` only), which is another sign the singular framing has slipped.
- **Fix:** Rename to the aggregate it represents. Preferred: `FileIssueCounts` (plural + role-tagged). Acceptable: `FileSummary`, `FileStats`. Avoid `FileIssues` alone — still reads as "the issues for the file" (a slice), not a counts struct.
  ```
  sarif.FileIssue          → sarif.FileIssueCounts
  // call sites: TopFiles return type, any iteration variables
  ```
  Grep map:
  ```
  rg -l 'sarif\.FileIssue|\[\]FileIssue|TopFiles' --type go
  # pkg/sarif/aggregates*.go, pkg/sarif/*_test.go, likely pkg/view/leaderboard.go or pkg/view/pickview.go
  ```
- **Tier:** 🟡 P2

---

## Looked at, not flagged

- **`state.Headline` (func returning string) vs `view.Headline` (struct)** — different packages, different domains (one is a sentence about the diff; the other is a renderable section). Acceptable polysemy at the package boundary.
- **`sarif.Run` vs `state.Run`** — same word, very different referents, but each is unambiguous inside its package and the cross-package call sites disambiguate via the `pkg.` prefix.
- **`pkg/cluster.Run([]Input) []Cluster`** — `Run` here is a verb (execute the clusterer); the function's body matches. Not a noun collision in callers (`cluster.Run(...)` reads correctly).
- **`internal/lineread.Read`** — `lineread.Read(br)` predicts "read a line from `br`", body reads exactly that.
- **`pkg/state/state.go` containing `Dir/Path/Load/Save/Reset/Append/RunFromReport`** — file basename matches package lifecycle; not a dumping ground.
- **Hygiene packages (`status`, `metrics`, `tally`, `scene`)** — each package's basename matches its `# fo:<name>` header sentinel and the parsed root type. Strong self-documenting structure.
- **Tests** — all sampled `Test<Foo>` names target real `Foo` symbols (`TestExtractTopUserFrame_*`, `TestClassify*`, `TestHeadline_*`, `TestMakeClusterID_*`).
- **Wrappers (`pkg/wrapper/wrap*`)** — file/package basenames match the tool they adapt (cover, jscpd, gobench, archlint, diag, leaderboard). Honest.

## Verdict

3 findings, all P2 cosmetic. Repo's naming hygiene is high. The two highest-impact renames are F1 (`Ruleset` → `Set`, propagates to one caller package) and F3 (`FileIssue` → `FileIssueCounts`, propagates to the leaderboard view). F2 is single-call-site if `AppendMetrics` is only invoked from `cmd/fo` and one wrapper.

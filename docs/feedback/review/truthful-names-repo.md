# truthful-names — repo review

RUN_ID: 7858b3a8ea1b
Scope: project (whole fo repo, HEAD `28a52d1`)
Linter: `truthful-names` (lens: clarity)

## Findings

### 1. [F1] `pkg/suppress/match.go:7-13` — terminology-drift

- **Diagnosis:** `suppress.Ruleset` and its field `.Rules` name a concept ("rule")
  that has no other surface anywhere in the package or its callers.
- **Why:** The package is `suppress`, the on-disk format is `.fo/ignore`, the
  parsed unit returned by `Parse` is `Suppression`, and the only other exported
  symbols are `Suppression.Expired`/`.Matches`/`.Format`. `Ruleset`/`Rules`
  introduces a second vocabulary for the same thing a reader has to privately
  translate ("a Ruleset holds Rules, but a Rule here is actually a
  Suppression"). The caller side reinforces the drift instead of correcting it:
  `cmd/fo/suppress.go` names its own loader `loadSuppressRuleset`, propagating
  "rule" language one layer further from the one real noun, `Suppression`.
- **Evidence:**
  ```go
  // pkg/suppress/match.go:5-14
  // Ruleset is an ordered list of suppressions loaded from .fo/ignore.
  // The zero value is empty and matches nothing.
  type Ruleset struct {
  	Rules []Suppression
  }

  // NewRuleset wraps parsed suppressions into a Ruleset.
  func NewRuleset(rs []Suppression) *Ruleset {
  	return &Ruleset{Rules: rs}
  }
  ```
  ```go
  // cmd/fo/suppress.go:45,67
  func loadSuppressRuleset(r *report.Report, path string, stderr io.Writer) *suppress.Ruleset {
      ...
      return suppress.NewRuleset(rules)
  }
  ```
- **Fix:** Rename the type/field/constructor to the package's own noun.
  `Ruleset` → `Set` (or `Suppressions`); `.Rules` → `.Items` (or `.List`);
  `NewRuleset` → `NewSet`. Call-site fan-out is small and contained:
  `pkg/report/filter.go` (`ApplyFilter`, `classifyFinding` both take
  `*suppress.Ruleset`) and `cmd/fo/suppress.go` (`loadSuppressRuleset`).
  ```
  rg -l 'suppress\.Ruleset|NewRuleset|SuppressRuleset' --type go
  ```
- **Tier:** borderline — cosmetic, single-package blast radius, but the name
  has now propagated into a second file (`cmd/fo/suppress.go`) since it was
  first flagged; left alone it keeps spreading.

---

### 2. [F2] `pkg/state/metrics_history.go:101` — imprecise-function-name

- **Diagnosis:** `AppendMetrics` reads as "add samples at the end of the
  file"; the body prepends, evicts, and rewrites the whole envelope.
- **Why:** A reader calling `state.AppendMetrics(path, samples)` from
  `cmd/fo/render.go:196` reasonably expects an incremental write. What
  actually happens: load the full `MetricsFile`, prepend a new `MetricsRun`
  to the front of `hist.Runs` (newest-first, per the type's own doc comment),
  truncate anything past `MaxMetricsHistory`, then atomically rewrite the
  entire file. That's three concerns ("prepend", "evict", "full read-modify-
  write") that "Append" doesn't carry, and "prepend" is the literal opposite
  of what "append" means for a slice.
- **Evidence:**
  ```go
  // pkg/state/metrics_history.go:98-109
  // AppendMetrics loads existing history, prepends a new run with the
  // current samples, trims to MaxMetricsHistory, and writes the envelope
  // back. Replaces the prior overwrite-only SaveMetrics (#258).
  func AppendMetrics(path string, samples []MetricSample) error {
  	hist, err := LoadMetricsHistory(path)
  	if err != nil {
  		return err
  	}
  	hist.Version = MetricsSchemaVersion
  	hist.Runs = append([]MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: samples}}, hist.Runs...)
  	if len(hist.Runs) > MaxMetricsHistory {
  		hist.Runs = hist.Runs[:MaxMetricsHistory]
  	}
  ```
  The function's own doc comment already spells out "prepends... trims...
  writes back" — the name just never caught up to that description.
- **Fix:** Rename to a verb that matches the doc comment, e.g.
  `RecordMetricsRun` (mirrors the existing `RunFromReport` vocabulary) or
  `RotateMetrics` (foregrounds the eviction). Single production call site:
  `cmd/fo/render.go:196`.
  ```
  rg -l 'AppendMetrics' --type go
  # pkg/state/metrics_history.go, pkg/state/metrics_history_test.go, cmd/fo/render.go
  ```
- **Tier:** borderline — one call site, mechanical rename, but the doc
  comment vs. name gap is the kind of thing that misleads a skim-reader
  specifically because the comment sounds authoritative and correct.

---

### 3. [F3] `pkg/sarif/aggregates.go:9` — terminology-drift

- **Diagnosis:** `FileIssue` names a per-file count aggregate, not an issue.
- **Why:** Everywhere else in `pkg/sarif`, "issue" (via `Result`) denotes one
  finding at one location. `FileIssue` breaks that convention inside the same
  package: it's `{File, IssueCount, ErrorCount, WarnCount}` — a rollup row for
  a leaderboard, with zero fields describing an individual issue. A reader who
  has just seen `sarif.Result` (one issue) then sees `[]FileIssue` returned by
  `TopFiles` and reasonably expects a list of individual issues grouped by
  file, not one row per file with counts.
- **Evidence:**
  ```go
  // pkg/sarif/aggregates.go:8-17
  // FileIssue represents an issue in a specific file for leaderboard rendering.
  type FileIssue struct {
  	File       string
  	IssueCount int
  	ErrorCount int
  	WarnCount  int
  }

  // TopFiles returns files sorted by issue count (descending).
  func TopFiles(doc *Document, limit int) []FileIssue {
  ```
  The struct's own field `IssueCount` on a type called `FileIssue` is the
  tell: a genuine single issue wouldn't need to count issues.
- **Fix:** Rename to name the aggregate: `FileIssueCounts` or `FileSummary`.
  Low-risk: `TopFiles`/`FileIssue` currently has no production caller —
  `rg` finds it only in `pkg/sarif/aggregates.go` and
  `pkg/sarif/aggregates_external_test.go`, so the rename's blast radius is one
  package plus its own test file (worth flagging to a human separately as a
  possible dead-code candidate, but out of scope for this linter).
  ```
  rg -l 'FileIssue|TopFiles' --type go
  # pkg/sarif/aggregates.go, pkg/sarif/aggregates_external_test.go — no other hits
  ```
- **Tier:** borderline — same drift class as F1, contained blast radius,
  arguably moot if the type is genuinely unused in production and gets
  removed instead of renamed.

## Looked at, not flagged

Swept the top-PageRank symbols (`internal/lineread.Read` 0.104, `pkg/suppress`
0.070, `pkg/report` 0.064, `pkg/sarif` 0.048), all 24 exported receiver methods
repo-wide, every package basename under `pkg/`/`internal/` (none generic), the
newest branch code (`pkg/state/fulllog.go`, `cmd/fo/state.go`'s
`recordFullLog`/`attachDiff`, fo-w1f tee-to-log), `pkg/multiplex` (post-move
from `pkg/report`), `pkg/scene`, `pkg/cluster`, `pkg/view/pickview.go`, and
test names across the packages touched by the last 20 commits
(`pkg/state/{fulllog,runlog,snapshot}_test.go`, `cmd/fo/{trend,explain}_test.go`).
All matched their bodies. `wrapdiag`'s two `Convert` symbols (package-level
wrapper + `(*diag).Convert` method) are a deliberate delegation, not a
collision — documented in `DiagOpts`'s comment as bypassing `*flag.FlagSet`
plumbing for the v2 CLI path.

## Note on prior run

A prior pass (2026-05-17, `bd775e303d86`) flagged these same three symbols.
All three are still unfixed in the current tree — re-verified against HEAD
rather than assumed carried-forward. No new findings surfaced beyond these.

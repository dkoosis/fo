# truthful-names — repo

scope: project · mode: report · run: f62c7fc3af14 · date: 2026-05-16

## Verdict

🟢 Names are largely honest. Packages are domain-specific (no `util`/`common`/`helpers`); file basenames match contents; most top-PageRank symbols predict their bodies. Two minor findings below; the rest of the surface reads clean.

| Tier | Count | Worst |
|---|---|---|
| P1 receiver/function mismatch | 0 | 🟢 |
| P1 package generic basename | 0 | 🟢 |
| P2 file/module | 1 | 🟡 |
| P2 test mismatch | 0 | 🟢 |
| P3 boolean-trap / param | 1 | 🟡 |

Overall: **🟡** (driven by one mildly-generic exported type name on the highly-imported `pkg/state` surface).

---

## Findings

### 1. [F1] `pkg/state.Item`

- **Pattern:** `imprecise-function-name` (applied to a type — generic noun on a public API)
- **Symbol:** `github.com/dkoosis/fo/pkg/state.Item`
- **Predicted from name:** "an item" — a row of something unspecified. Reader has to read fields to learn it's a diff classification.
- **Actual from body:** A single classified finding-or-test entry in the diff envelope. Carries `Class` (new/persistent/resolved/regressed/flaky), `Fingerprint`, `Severity`, `PriorSeverity`. `pkg/state/diff.go:25`.
- **Evidence:**
  ```go
  // Item is one classified entry in the diff envelope. Resolved entries
  // carry the prior fingerprint+severity; new/regressed/persistent carry
  // the current snapshot.
  type Item struct {
      Fingerprint   string          `json:"fingerprint"`
      RuleID        string          `json:"rule_id,omitempty"`
      ...
      Class         Class           `json:"class"`
  }
  ```
  Used everywhere as `[]Item` inside `Diff` and `Envelope` (`pkg/state/diff.go:40-55`, `pkg/state/headline.go:46-58`). Re-exported through `report.DiffItem` (`cmd/fo/state.go:67`) — so the consumer side already needed a more specific name.
- **Why it matters:** `state` is the 13th-highest pagerank package and `Item` is the JSON shape that LLM/JSON consumers see. The downstream renderer code (`cmd/fo/state.go`) had to invent `report.DiffItem` for clarity, which is the tell.
- **Fix:** rename `state.Item` → `state.DiffEntry` (or `ClassifiedFinding`). Field tag `json` stays the same (entries are inside named slices `new`/`resolved`/… so the type name isn't on the wire).
  - grep map:
    ```
    \bstate\.Item\b      → state.DiffEntry
    \[\]Item\b           → []DiffEntry        # inside pkg/state only
    type Item struct     → type DiffEntry struct
    func.*Item\)         → func.*DiffEntry)
    ```
  - Touch: `pkg/state/diff.go`, `pkg/state/headline.go`, `pkg/state/diff_test.go`, `pkg/state/state_test.go`, `cmd/fo/state.go` (the conversion site).

### 2. [F2] `cmd/fo/fswatch.go` parameter `name` in `shouldIgnoreDir`

- **Pattern:** `imprecise-function-name` (parameter naming variant)
- **Symbol:** `cmd/fo/fswatch.go:25 shouldIgnoreDir(name string) bool`
- **Predicted from name:** `name` could be a full path or a basename.
- **Actual from body:** Strictly a directory **basename** — compared against `defaultIgnoreDirs` (`"vendor"`, `"node_modules"`, …) and the leading `.`-prefix rule. Passing a full path silently fails the membership check.
- **Evidence:**
  ```go
  func shouldIgnoreDir(name string) bool {
      if name == "" || name == "." {
          return false
      }
      if slices.Contains(defaultIgnoreDirs, name) {
          return true
      }
      // hidden-dir check on name[0] == '.'
  ```
- **Fix:** rename parameter `name` → `basename` (doc-comment already uses that word). Single-file change.
  - grep map: `shouldIgnoreDir(name` → `shouldIgnoreDir(basename`; update body refs.

---

## Notes (sub-threshold, not findings)

- `pkg/view/multiples.go` — file name shortens the only topic inside (`SmallMultiples` ViewSpec + renderer). Borderline; the package doc and the type name carry the precision, and there's no other `multiples*.go` to confuse it with. Don't flag.
- Two `Headline` types co-exist (`state.Headline` returns a summary string; `view.Headline` is a ViewSpec variant). Distinct packages, distinct roles, each name is locally accurate. Don't flag.
- `pkg/testjson.processLine` and `pkg/testjson.processEventLine` are byte-identical (`pkg/testjson/parser.go:45,156`). Naming is interchangeable; the real defect is duplication — out of scope for truthful-names, belongs to `/review dedup` or a manual consolidation.
- `pkg/sarif.Stats` vs `pkg/testjson.Stats` — same word for two different aggregates, but in distinct packages with distinct domains (SARIF findings vs Go test outcomes). Idiomatic Go; not terminology drift.
- Module path `github.com/dkoosis/fo` — short, but `fo` is the established binary name and matches the `fo` CLI noun used throughout README/CLAUDE.md. Not uninformative once you know the project; don't flag (rule explicitly excludes established reputations).

## Don't-flag log

- `Get`-style getters absent — N/A.
- Receivers all map cleanly to the verb their method names promise (`(*aggregator).ProcessEvent`, `(*aggregator).Results`, `(*Builder).AddResultWithFix` — every body matches).
- Test names match tested code (`TestFingerprint_*`, `TestScore_*`, `TestParseStream_*`, `TestPickView_*` all exercise the named function).

# domain-model — repo-wide pass

Scope: whole fo repo (all packages, one condensed pass). Mode: report. RUN_ID: 7858b3a8ea1b.

**Headline:** fo's domain model is mature. Severity, TestOutcome, Status, State,
Class, BeatKind, Mode, and OutputKind are all named types with constants and
boundary validators — the common primitive-obsession shapes (`map[string]any`,
multi-bool signatures, `duration-as-int`) are absent. The findings below are the
few places where a type that *already exists* is downgraded to a bare `string` at
a boundary, plus one enum concept (SARIF level) that never got a type despite
being a closed set read in four places. One structural clump in `pkg/view`.

Scoring: Identifier types 🟢 · Enums 🟡 · Parameter shape 🟡 · Opaque shapes 🟢.
Overall 🟡 — driven by the two enum downgrades.

---

### 1. [F1] `pkg/report/report.go:111-118` — stringly-typed-enum

**Diagnosis:** `DiffItem` stores `Severity`, `PriorSeverity`, and `Class` as bare
`string`, even though `report.Severity` (a closed 3-value enum) is declared in the
*same file* and `state.Class` is a typed enum upstream. The typed value is cast
away at the conversion boundary.

**Why:** report already owns the `Severity` type — typing these fields costs zero
new imports and no new dependency edge. Leaving them `string` means the diff view
and JSON schema carry an untyped severity that no longer compares as a
`report.Severity`, and `Class` ("new"/"persistent"/"resolved"/"regressed"/"flaky")
has no type at all on the report side, so a typo in a hand-built `DiffItem`
compiles. The `string(it.Class)` / `string(it.Severity)` casts in
`cmd/fo/state.go:144-146` are the downgrade made visible.

**Evidence:**
`pkg/report/report.go:8-16` — the type that already exists:
```go
// Severity is the level of a static-analysis finding. The set is closed:
type Severity string
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)
```
`pkg/report/report.go:111-118` — the same concept, downgraded:
```go
type DiffItem struct {
	Fingerprint   string `json:"fingerprint"`
	RuleID        string `json:"rule_id,omitempty"`
	File          string `json:"file,omitempty"`
	Severity      string `json:"severity"`
	PriorSeverity string `json:"prior_severity,omitempty"`
	Class         string `json:"class"`
}
```
`cmd/fo/state.go:144-146` — the type cast away at the seam:
```go
			Severity:      string(it.Severity),
			PriorSeverity: string(it.PriorSeverity),
			Class:         string(it.Class),
```
`pkg/state/diff.go:10-17` shows `Class` is already a typed enum upstream
(`ClassNew`, `ClassPersistent`, `ClassResolved`, `ClassRegressed`, `ClassFlaky`).

**Fix:** Type the fields as `report.Severity` (no new import) and add a report-side
`type DiffClass string` with the five constants (report must not import state per
the one-way-edge comment on line 109, so mint the type in report; the CLI conversion
becomes `report.DiffClass(it.Class)` at the seam). This turns `cmd/fo/state.go`'s
bare `string(...)` casts into type-checked conversions.

**Tier:** action

---

### 2. [F2] `pkg/sarif/types.go:15-18` — stringly-typed-enum

**Diagnosis:** The SARIF result level is a closed 4-value set
(`error`/`warning`/`note`/`none`) read in at least four places, but it has no type:
the constants are *untyped* strings, the `Result.Level` field is bare `string`, and
`pkg/score.SeverityWeight` re-hardcodes the same literals instead of referencing the
constants.

**Why:** The concept threads through `checkLevel`, `TopFiles`' switch, `mapSeverity`,
and `score.SeverityWeight` as a bare string. Because the constants are untyped, a
caller can pass any `string` to `AddResult(level string, ...)` and only find out at
runtime via `checkLevel`. Worse, `score.SeverityWeight` switches on literal
`"error"`/`"warning"`/`"note"` — a second, drift-prone copy of the value set
divorced from `sarif.LevelError` et al. A `type Level string` would make the level
a first-class enum and let `AddResult` reject bad levels at compile time for
in-tree callers.

**Evidence:**
`pkg/sarif/types.go:15-18` — untyped constants, no `type Level`:
```go
	LevelError   = "error"
	LevelWarning = "warning"
	LevelNote    = "note"
	LevelNone    = "none"
```
`pkg/sarif/types.go:49` — the field is a bare string:
```go
	Level     string     `json:"level"` // "error", "warning", "note", "none"
```
`pkg/sarif/builder.go:42-47` — runtime validation that a type would move to compile time:
```go
func checkLevel(level string) error {
	switch level {
	case LevelError, LevelWarning, LevelNote, LevelNone:
		return nil
	}
	return fmt.Errorf("%w: %q", errInvalidLevel, level)
}
```
`pkg/score/score.go:32-37` — the divorced literal copy (does not use the constants):
```go
	case "error":
		return SeverityWeightError
	case "warning":
		return SeverityWeightWarning
	case "note":
		return SeverityWeightNote
```

**Fix:** `type Level string` in pkg/sarif with the four constants typed; type
`Result.Level` and `checkLevel`/`AddResult` parameters as `Level`. `pkg/score`
cannot import sarif (sarif→score is the existing edge), so leave `SeverityWeight`
taking a string but have it consume `sarif.Level` values passed by the caller.
Borderline because the field is a JSON wire shape (external SARIF spec) — runtime
validation of *inbound* JSON stays necessary regardless.

**Tier:** borderline

---

### 3. [F3] `pkg/view/stream.go:14-25` — parameter-clump

**Diagnosis:** `(t theme.Theme, width int)` — and its extension `(..., mode Mode)`
— co-occur across the exported render surface (`RenderReport`, `RenderReportMode`,
`RenderReportModeWithExpand`, `RenderStream`, `RenderStreamMode`, `Render`) and the
unexported helpers (`handleSnapshot`, `writeSnapshot`, `flushStream`,
`renderDelta`, `renderLeaderboard`, `renderSmallMultiples`). It is an unnamed
render context.

**Why:** The three progressive-overload variants (`RenderReport` →
`RenderReportMode` → `RenderReportModeWithExpand`) differ only by one trailing
param each — the exact drift the rule warns about: add a rendering knob (a second
density, a palette override) and every signature in the family grows. A
`type RenderContext struct { Theme theme.Theme; Width int; Mode Mode }` names the
sub-aggregate and lets the family collapse toward one signature with option
defaults.

**Evidence:** `pkg/view/stream.go:14-25`:
```go
func RenderReport(w io.Writer, r report.Report, t theme.Theme, width int) error {
	return RenderReportMode(w, r, t, width, ModeHuman)
}
func RenderReportMode(w io.Writer, r report.Report, t theme.Theme, width int, mode Mode) error {
	return RenderReportModeWithExpand(w, r, t, width, mode, expandSet{})
}
func RenderReportModeWithExpand(w io.Writer, r report.Report, t theme.Theme, width int, mode Mode, expand expandSet) error {
```
Same `t theme.Theme, width int` pair recurs at `stream.go:47,58,87,99,108` and
`render.go:21` (`func Render(spec ViewSpec, t theme.Theme, width int) string`).

**Fix:** Introduce `type RenderContext struct { Theme theme.Theme; Width int; Mode Mode }`
and thread it. Borderline: the overload family is a deliberate
backward-compat idiom, and the pair is only two fields — a human should weigh the
API churn against the readability gain.

**Tier:** borderline

---

### 4. [F4] `pkg/testjson/types.go:29-33` — stringly-typed-enum

**Diagnosis:** The `go test -json` action is a fixed vocabulary
(`pass`/`fail`/`skip`/`output`/`build-output`/`build-fail`/...) but only half-modeled:
`actionPass`/`actionFail`/`actionSkip` are declared as *untyped* string constants,
`TestEvent.Action` is a bare `string`, and the aggregator compares it against those
constants *mixed with* bare literals (`"output"`, `"build-output"`, `"build-fail"`).

**Why:** Two comparison styles for one concept — named constant in some cases, raw
literal in others — invites a typo like `"buildfail"` that compiles and silently
drops build events. A `type Action string` with the full set as constants would make
the switch exhaustive-looking and typo-proof.

**Evidence:** `pkg/testjson/types.go:27-33` — half-typed, explicitly untyped:
```go
// TestEvent.Action values from `go test -json`. Same string values as the
// Status constants, but used for comparing untyped Action strings.
const (
	actionPass = "pass"
	actionFail = "fail"
	actionSkip = "skip"
)
```
`pkg/testjson/parser.go:256,273` — constants and bare literals mixed:
```go
	if e.Action == "build-output" || e.Action == "build-fail" {
```
```go
	case "output":
```
(`parser.go:262-267` and `funcresults.go:39-45` switch on `actionPass/Fail/Skip`.)

**Fix:** `type Action string` in types.go with constants for the full documented
set; type `TestEvent.Action` as `Action`; replace the bare literals with named
constants. Borderline: `Action` is an external wire field (go-test-json), so the
JSON tag stays and inbound values still arrive as arbitrary strings — the win is
internal consistency and typo-safety at the comparison sites, not boundary validation.

**Tier:** borderline

---

trixi log-skill domain-model findings 4 --run-id "7858b3a8ea1b"

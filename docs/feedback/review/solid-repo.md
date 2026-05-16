# SOLID review — fo (repo)

run_id: f62c7fc3af14
linter: solid (SRP / LSP / ISP)
scope: project
date: 2026-05-16

## Orientation

The fo codebase is deliberately interface-sparse and method-light. The only
production interface is `pkg/view.ViewSpec`, an intentionally closed sum-type
satisfied via an unexported `isViewSpec()` marker — adding a variant requires
editing `Render`'s exhaustive switch, by design. `pkg/wrapper/doc.go` documents
the explicit decision: no `Wrapper` interface, no registry, dispatched via a
`switch` in `cmd/fo/main.go`.

Implications for SOLID:

- **LSP:** no multi-impl production interfaces → no Penguin.Fly() smells available.
- **ISP:** `ViewSpec` has one method (a marker) and is satisfied by every
  variant — not "fat." No other production interface to split.
- **SRP:** most "types" are value-only DTOs or builders. A few mix parser-output
  shape with renderer-facing conversion methods, worth calling out as small
  design improvements.

Scoring (overall = worst tier):

| Tier | Result |
|------|--------|
| P1 LSP | 🟢 0 |
| P1 SRP | 🟡 2 — parser DTO with render methods (Tally), trivial codec hop on parser DTO (Status) |
| P2 ISP | 🟢 0 |

Overall: 🟡 (1-2 SRP findings worth doing; no LSP or ISP defects).

---

## Findings

### 1. [F1] `pkg/tally.Tally` — parser DTO carries a renderer method

- **Symbol:** `pkg/tally.Tally`
- **Principle:** SRP (`srp-mixed-concerns`)
- **Evidence:** `Tally` is the parser output (`Parse(io.Reader) (Tally, error)`)
  but also owns `RenderLLM(w io.Writer) error` — a renderer that produces
  plain-text aligned output. Two concerns on one type: *parse-result DTO* and
  *terminal/LLM rendering*. Compare with the architectural rule documented in
  `.claude/rules/CLAUDE.md`: "Report is the IR. Parsers produce it; renderers
  consume it." `Tally` violates that locally — it both *is* the parsed data and
  *renders* itself.

- **Code:** `pkg/tally/tally.go:46`, `:168`, `:181`

  ```go
  type Tally struct {
      Tool string `json:"tool,omitempty"`
      Rows []Row  `json:"rows"`
  }

  func (t Tally) ToLeaderboard() view.Leaderboard { ... }    // codec → IR
  func (t Tally) RenderLLM(w io.Writer) error    { ... }     // RENDER
  ```

- **Why it matters:** `RenderLLM` is called once, from `cmd/fo/main.go:365`, in
  parallel with the human path that goes through `view.Leaderboard` →
  `pkg/view`. If a second renderer concern lands on `Tally` (json output, csv,
  …) the type fans out further along a layer it shouldn't own.

- **Fix:** keep `Tally` and `ToLeaderboard` on the type (the codec hop is
  cohesive with the data shape). Move `RenderLLM` to either:
  - a free function in `pkg/view` (`view.RenderLeaderboardLLM(view.Leaderboard, io.Writer)`), so all LLM rendering lives in one package; **or**
  - a free function in `pkg/tally` (`func RenderLLM(t Tally, w io.Writer) error`) — removes the *method on the DTO* shape without moving packages.

  Preferred: the first — it matches the existing pkg/view ownership of
  presentation.

---

### 2. [F2] `pkg/status.Status` — parser DTO carries a view-shape codec

- **Symbol:** `pkg/status.Status`
- **Principle:** SRP (`srp-mixed-concerns`, minor)
- **Evidence:** `Status` is the parser output of `# fo:status` hygiene input;
  `ToViewRows()` converts to `ViewRow`, a renderer-facing shape, with the
  declared rationale "mirrors view.StatusRow so pkg/view doesn't need to
  import pkg/status." That comment names the design tension. The conversion is
  a trivial field copy and the codec hop is owned by the parser package only
  to dodge a one-way import.

- **Code:** `pkg/status/status.go:171-185`

  ```go
  // ViewRow is the renderer-facing shape; mirrors view.StatusRow so
  // pkg/view doesn't need to import pkg/status.
  type ViewRow struct { State, Label, Value, Note string }

  func (s Status) ToViewRows() []ViewRow {
      out := make([]ViewRow, len(s.Rows))
      for i, r := range s.Rows {
          out[i] = ViewRow{ ... }
      }
      return out
  }
  ```

- **Why it matters:** This is borderline — by lintbrush rules, "trivial single
  method" counts as a *don't-flag*. Listed here because the duplicated shape
  (`Status.ViewRow` vs `view.StatusRow`) is the actual smell, and the
  conversion exists only to bridge it. The SRP cost is minor; the duplication
  cost is what hurts.

- **Fix:** introduce a shared types package (e.g. a tiny `pkg/hygiene/rows`) or
  flip the import direction (pkg/view imports pkg/status's `Row` directly).
  Either eliminates `ViewRow` and `ToViewRows`. Keep `Status` as a pure parser
  DTO.

---

### 3. [F3] `pkg/state.Item` — unexported back-pointer field is dead

- **Symbol:** `pkg/state.Item` (field `report *report.Finding`)
- **Principle:** SRP / "type tells the truth" (not strictly SRP but in the
  spirit of `srp-mixed-concerns` — the type's shape implies a relationship
  that isn't real)
- **Evidence:** `pkg/state/diff.go:32` declares an unexported `report` field on
  `Item` with comment "back-pointer; not serialized." `rg "\.report\b"` across
  `pkg/state/` returns only the field declaration and `makeItem` assignment.
  No call site reads it. The field carries weight in the type's mental model
  but does nothing.

- **Code:** `pkg/state/diff.go:25-33`, `:205-218`

  ```go
  type Item struct {
      ...
      Class         Class           `json:"class"`
      report        *report.Finding // unexported back-pointer; not serialized
  }

  func makeItem(fp string, sev, prior Severity, c Class, f *report.Finding) Item {
      it := Item{ ..., report: f }
      if f != nil {
          it.RuleID = f.RuleID
          it.File = f.File
      }
      return it
  }
  ```

- **Why it matters:** dead state is a small SRP signal — the type pretends to
  own a relationship to `report.Finding` that no consumer uses. Removing it
  simplifies `makeItem` (drop the `*report.Finding` parameter) and clarifies
  that `Item` is a flat row, not a graph node.

- **Fix:** drop the `report` field and the `f *report.Finding` parameter on
  `makeItem`. The two reads (`f.RuleID`, `f.File`) become direct arguments. If
  a future caller needs the back-pointer, add it then with a real consumer.

---

## Not flagged (and why)

- **`pkg/view.ViewSpec`** — closed sum-type, not an ISP target. Marker method
  is intentional; "exhaustive switch in Render" is the documented contract.
- **Wrapper convert-structs (`archlint`, `jscpd`, `diag`)** — each has one
  `Convert(io.Reader, io.Writer) error` method, single-concern. The deliberate
  *absence* of a `Wrapper` interface (per `pkg/wrapper/doc.go`) is also fine —
  there are only three impls, all called from the same switch.
- **`pkg/sarif.Builder`** — methods are all aspects of one cohesive role
  (build a SARIF doc); not SRP-mixed.
- **`cmd/fo/main.go` (1318 LOC)** — function-style, no god-struct. SOLID is
  type-level; this is a separate concern (file-length / `lintbrush:file-splitting`).
- **`report.DiffSummary` vs `state.Diff` near-duplication** — a real design
  issue but a layering/duplication smell, not SOLID at the type-method level.
  Defer to `/review arch` or a duplication pass.

---

## Summary

Three findings, all SRP-flavored, all small. No LSP, no ISP. The codebase
mostly follows a function-style layout that side-steps SOLID's classic traps;
the remaining smells are parser DTOs that grew renderer methods (F1, F2) and
one dead field (F3). F1 is the only one I'd ship as a standalone change.

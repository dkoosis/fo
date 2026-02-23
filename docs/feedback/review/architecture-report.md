# Architecture Review

**Date:** 2026-02-18
**Snipe index:** 175 symbols, 524 refs, 98 calls
**Scope:** 8 packages, 2190 LOC (non-test), 626 LOC (test)
**Module:** github.com/dkoosis/fo
**Go version:** 1.24.0
**Direct dependencies:** lipgloss, x/term (2 total)

---

## Conformance

**Status:** No `.go-arch-lint.yml` or `.go-arch-lint-target.yml` found.

No enforced architectural rules exist. The codebase has no formal layering constraints defined. At 2.2k LOC across 8 packages this is not urgent, but worth establishing before the codebase grows.

**Enforced rules:** N/A (no config)
**Target rules:** N/A (no config)
**Tech debt exclusions:** 0

---

## Dependency Topology

### Internal Dependency Graph

```
cmd/fo ──> internal/detect
       ──> pkg/mapper
       ──> pkg/pattern
       ──> pkg/render
       ──> pkg/sarif
       ──> pkg/testjson

pkg/mapper ──> pkg/pattern
           ──> pkg/sarif
           ──> pkg/testjson

pkg/render ──> pkg/pattern
```

### Stability Analysis

| Package | Ca (fan-in) | Ce (fan-out) | Instability | Assessment |
|---------|-------------|--------------|-------------|------------|
| pkg/pattern | 3 | 0 | 0.00 | Stable foundation (leaf) |
| pkg/sarif | 2 | 0 | 0.00 | Stable foundation (leaf) |
| pkg/testjson | 2 | 0 | 0.00 | Stable foundation (leaf) |
| internal/detect | 1 | 0 | 0.00 | Stable (leaf) |
| pkg/mapper | 1 | 3 | 0.75 | Unstable — expected for orchestration |
| pkg/render | 1 | 1 | 0.50 | Moderate — depends only on pattern |
| cmd/fo | 0 | 6 | 1.00 | Unstable — expected for entry point |
| internal/version | 0 | 0 | 0.00 | Orphan — zero dependents, zero deps |

### Danger Zone

No packages in the danger zone. The highest Ca is 3 (pkg/pattern), well under the threshold of 10. No package simultaneously has high fan-in and high instability.

### Cross-Package Call Coupling

| Caller -> Callee | Calls |
|------------------|-------|
| cmd/fo -> pkg/render | 8 |
| pkg/mapper -> pkg/sarif | 8 |
| pkg/render -> pkg/pattern | 5 |
| cmd/fo -> pkg/sarif | 4 |
| pkg/mapper -> pkg/testjson | 4 |
| cmd/fo -> pkg/mapper | 2 |
| cmd/fo -> internal/detect | 1 |
| cmd/fo -> pkg/testjson | 1 |

No tightly coupled pairs (all under 50 calls). Maximum is 8 calls between cmd/fo->pkg/render and pkg/mapper->pkg/sarif.

### External Dependencies

Only 2 direct external dependencies (lipgloss, x/term), both confined to their expected packages:
- `github.com/charmbracelet/lipgloss` — used only in pkg/render
- `golang.org/x/term` — used only in cmd/fo

All other imports are stdlib. This is excellent dependency hygiene.

---

## API Surface

| Package | Exported | Funcs | Methods | Types | Vars | Consts | Tier |
|---------|----------|-------|---------|-------|------|--------|------|
| pkg/sarif | 30 | 10 | 6 | 14 | 0 | 0 | Yellow |
| pkg/pattern | 21 | 0 | 5 | 11 | 0 | 5 | Yellow |
| pkg/render | 16 | 7 | 3 | 6 | 0 | 0 | Yellow |
| pkg/testjson | 10 | 3 | 2 | 5 | 0 | 0 | Green |
| internal/detect | 5 | 1 | 0 | 1 | 0 | 3 | Green |
| internal/version | 3 | 0 | 0 | 0 | 3 | 0 | Green |
| pkg/mapper | 2 | 2 | 0 | 0 | 0 | 0 | Green |

### Notes on Yellow-Tier Packages

**pkg/sarif (30 exports):** 14 of the 30 are struct types representing the SARIF 2.1.0 spec (Document, Run, Result, Location, Region, etc.). These are data transfer types necessarily public for the domain. The remaining 16 are reader functions (Read, ReadBytes, ReadFile, GroupByFile, GroupByRule, etc.) and a builder. Cohesion is high -- everything relates to SARIF I/O. No action needed.

**pkg/pattern (21 exports):** 11 are struct types (5 pattern types + their Item companions), 5 are PatternType constants, 1 is the Pattern interface, 5 are Type() methods. This is the expected expansion of a sealed type hierarchy. Cohesion is high.

**pkg/render (16 exports):** 6 types (3 renderers + Renderer interface + Theme + related), 7 constructor/factory functions, 3 methods. Clean renderer implementations. No issues.

### Interface Health

| Interface | Implementations | Assessment |
|-----------|----------------|------------|
| Pattern (pkg/pattern) | 5 (Summary, Sparkline, TestTable, Comparison, Leaderboard) | Healthy polymorphism |
| Renderer (pkg/render) | 3 (Terminal, LLM, JSON) | Healthy polymorphism |

Both interfaces serve clear architectural roles. Pattern is the data abstraction between mappers and renderers. Renderer is the output strategy. No dead interfaces. No single-implementation over-abstractions.

---

## Package Health Heatmap

| Package | LOC | Files | Exports | Max File LOC | Instability | Test Files | Tier |
|---------|-----|-------|---------|-------------|-------------|------------|------|
| pkg/render | 674 | 5 | 16 | 289 | 0.50 | 0 | Yellow |
| pkg/sarif | 382 | 3 | 30 | 206 | 0.00 | 1 | Green |
| pkg/mapper | 372 | 2 | 2 | 213 | 0.75 | 0 | Green |
| pkg/testjson | 311 | 3 | 10 | 211 | 0.00 | 1 | Green |
| cmd/fo | 257 | 1 | N/A | 257 | 1.00 | 1 | Green |
| pkg/pattern | 106 | 6 | 21 | 22 | 0.00 | 0 | Green |
| internal/detect | 80 | 1 | 5 | 80 | 0.00 | 1 | Green |
| internal/version | 8 | 1 | 3 | 8 | 0.00 | 0 | Green |

### Scoring Rationale

All packages are well under the Red thresholds (LOC > 3000, Files > 15, Exports > 40, Max File > 800).

**pkg/render** is the only Yellow-tier package:
- 0 test files -- the only pkg/ package with exports and no tests (pkg/pattern and pkg/mapper also lack tests, but render has the highest LOC at 674)
- Instability at 0.50 with 1 dependent (pkg_render depends on pkg/pattern; cmd/fo depends on pkg/render)

All others are Green on every metric.

---

## Structural Findings

### Orphan Packages

- **internal/version** -- Zero internal dependents. Zero references anywhere in the codebase. Contains 3 exported vars (Version, CommitHash, BuildDate) intended for ldflags injection, but no Go file imports this package. This is dead code. Candidate for removal or wiring into cmd/fo.

- **cmd/fo** -- Zero internal dependents. This is expected and correct for an entry point.

### God Packages

None. The largest package (pkg/render at 674 LOC, 16 exports, Ca=1) is well below god-package thresholds.

### Test Coverage Gaps

| Package | LOC | Test Files | Test LOC | Coverage Status |
|---------|-----|------------|----------|-----------------|
| cmd/fo | 257 | 1 | 347 | Covered (e2e tests) |
| pkg/sarif | 382 | 1 | 107 | Covered (builder tests) |
| pkg/testjson | 311 | 1 | 123 | Covered (parser tests) |
| internal/detect | 80 | 1 | 49 | Covered |
| **pkg/render** | **674** | **0** | **0** | **No tests** |
| **pkg/mapper** | **372** | **0** | **0** | **No tests** |
| **pkg/pattern** | **106** | **0** | **0** | **No tests** |
| **internal/version** | **8** | **0** | **0** | **Dead code** |

Three active packages (render, mapper, pattern) totaling 1152 LOC have zero unit tests. The 347-line e2e test in cmd/fo likely exercises these paths end-to-end, but there is no isolated unit test coverage for:
- Terminal/LLM/JSON rendering logic (289+233+47 LOC)
- SARIF-to-pattern and testjson-to-pattern mapping (159+213 LOC)
- Pattern data structures (106 LOC -- though these are mostly struct definitions)

### Migration Progress

N/A -- no `.go-arch-lint-target.yml` exists.

### Dependency Direction Compliance

The dependency graph is acyclic and follows a clean layered architecture:

```
Layer 0 (leaf):     pkg/pattern, pkg/sarif, pkg/testjson, internal/detect
Layer 1 (logic):    pkg/mapper (depends on L0), pkg/render (depends on L0)
Layer 2 (entry):    cmd/fo (depends on L0 + L1)
```

No upward dependencies. No cross-layer violations. No cycles. The design decision to make cmd/fo the sole orchestrator that wires mapper output into renderers is clean.

One note: cmd/fo imports pkg/sarif and pkg/testjson directly (not just through pkg/mapper). This is for the initial parse step before mapping -- architecturally valid since cmd/fo is the orchestrator.

---

## Scorecard

| Dimension | Score | Notes |
|-----------|-------|-------|
| Conformance | N/A | No arch-lint rules defined |
| Coupling | Green | 0 danger zone packages, max Ca=3, clean acyclic graph |
| API Surface | Green | 0 red-tier packages, yellow packages justified by domain types |
| Package Health | Yellow | pkg/render is yellow (no tests, 674 LOC), 3 packages lack tests |
| Structural | Yellow | 1 orphan (internal/version is dead code), 3 untested packages |
| **Overall** | **Yellow** | Clean architecture, minor hygiene issues |

---

## Actionable Findings

### 1. Remove or wire internal/version (dead code)
`internal/version` defines Version/CommitHash/BuildDate vars for ldflags but nothing imports it. Either:
- Wire it into cmd/fo (add `import "github.com/dkoosis/fo/internal/version"` and use the vars)
- Delete it if version embedding is not needed

### 2. Add unit tests for pkg/render, pkg/mapper
These are the two highest-LOC untested packages (674 + 372 = 1046 LOC). The e2e tests in cmd/fo provide some indirect coverage, but render logic (terminal formatting, LLM output, JSON serialization) and mapping logic (SARIF->patterns, testjson->patterns) would benefit from isolated tests.

### 3. Consider establishing .go-arch-lint.yml
At 2.2k LOC and 8 packages, the architecture is clean and manageable without formal rules. But codifying the layering now (while it's simple) prevents accidental violations as the codebase grows. The current dependency graph maps directly to a 3-layer model:
- **foundation:** pkg/pattern, pkg/sarif, pkg/testjson, internal/detect
- **logic:** pkg/mapper, pkg/render
- **entry:** cmd/fo

### Suggested Nug Candidates

- `kind: trap, name: "internal/version dead code"` -- unreferenced package, wasted cognitive overhead
- `kind: gap, name: "pkg/render untested"` -- highest-LOC package with zero unit tests
- `kind: gap, name: "pkg/mapper untested"` -- mapping logic untested in isolation

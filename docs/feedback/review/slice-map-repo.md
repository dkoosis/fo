# slice-map review: fo repo

RUN_ID: 7858b3a8ea1b  
Scope: project  
Mode: report

## Summary

Reviewed slice/map boundaries across 56 packages in fo. Found defensive-copy patterns well-applied in high-risk areas (testjson/parser streaming, state/headline nil normalization). One boundary-return violation flagged below; the pattern is currently safe in context but represents encapsulation leakage that could become unsafe if callers evolve.

---

### 1. [F1] `pkg/state/diff.go:156` — boundary-returns-internal-backing

**Diagnosis:** `priorRuns` returns a sub-slice view into the internal `File.Runs` array without copying.

**Why:** The function exports `older := prev.Runs[1:]`, giving the caller a mutable view into the producer's backing array. While current callers (classifyFinding, classifyTests) only read from `older` within the same Classify function call and don't retain it, the boundary is undefended. A future caller holding the slice while the File is garbage collected or modified would see stale or invalid data.

**Evidence:**
```go
// pkg/state/diff.go:151-160
func priorRuns(prev *File) (prior Run, older []Run) {
	if prev != nil && len(prev.Runs) > 0 {
		prior = prev.Runs[0]
		if len(prev.Runs) > 1 {
			older = prev.Runs[1:]
		}
	}
	return prior, older
}
```

**Fix:**
Copy the slice on return so the caller cannot mutate the producer's state:

```before
func priorRuns(prev *File) (prior Run, older []Run) {
	if prev != nil && len(prev.Runs) > 0 {
		prior = prev.Runs[0]
		if len(prev.Runs) > 1 {
			older = prev.Runs[1:]
		}
	}
	return prior, older
}
```

```after
func priorRuns(prev *File) (prior Run, older []Run) {
	if prev != nil && len(prev.Runs) > 0 {
		prior = prev.Runs[0]
		if len(prev.Runs) > 1 {
			older = append([]Run(nil), prev.Runs[1:]...)
		}
	}
	return prior, older
}
```

**Tier:** borderline

The pattern is currently safe because `older` is only consumed immediately within Classify and not retained by callers. However, defensive copying is the established pattern in fo (e.g., testjson/parser.go:390, 405 for panicOutput and failedTests), and this site breaks it. The cost of copying is minimal (typically 0-3 items) and guards against future regressions when refactoring.

---

## Findings summary

- **boundary-returns-internal-backing:** 1
- Total: 1 finding (borderline tier)

### Not flagged

- **Append aliasing:** No instances of append on sub-slices or shared references found.
- **Capacity retention:** No patterns of small slices long-term retained from large backing arrays detected.
- **Nil vs empty returns:** Consistent patterns observed (nonNil helper normalizes nil→[]Item{} for JSON contracts).
- **Map mutations during iteration:** Delete operations in testjson/parser are safe (happen on event handlers, not during iteration).
- **Pre-allocation:** make([]T, 0, cap) patterns applied consistently where iteration counts are known.

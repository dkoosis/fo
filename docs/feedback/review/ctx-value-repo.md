# ctx-value review — fo (repo scope)

- **Run:** f62c7fc3af14
- **Linter:** ctx-value (mode: report, scope: project)
- **Target:** /Users/vcto/Projects/fo
- **Date:** 2026-05-16

## Summary

No `context.WithValue` producers and no `ctx.Value(...)` consumers anywhere in the
Go source tree. `context.Context` appears ~19 times in the codebase, used strictly
for cancellation / deadline propagation (the intended use), never as a service
locator or value bag.

This is the ideal state for the ctx-value linter:

- No hidden-dependency smuggling through context.
- No untyped string keys.
- No scattered type assertions.
- No stacked WithValue chains.
- No mutable state stashed in context.

## Scoring

| Tier | Result |
|------|--------|
| P1 hidden deps | 🟢 0 |
| P1 fn-takes-ctx-reads-biz | 🟢 0 |
| P2 type-assert scattered | 🟢 0 |
| P2 untyped key | 🟢 0 |
| P3 stacked chain | 🟢 ≤2 (none) |

Overall: 🟢 clean.

## Evidence

```bash
$ rg -n 'context\.WithValue|ctx\.Value\(|\.Value\(' --type go
(no matches)

$ rg -n 'WithValue|\.Value\(' --type go
(no matches)

$ rg -n 'context\.Context' --type go | wc -l
19
```

All 19 `context.Context` occurrences are parameter passing for cancellation,
not value retrieval. Spot-checked sites (watch loop, stream parsers) confirm
ctx is used only for `<-ctx.Done()` / propagation.

## Findings

(none)

## Recommendation

No action. If future work adds request-scoped data (e.g. trace IDs for the
watch/server modes), follow the rules.md guidance: unexported key type,
typed accessor, reserve context for genuinely cross-cutting values.

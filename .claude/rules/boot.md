# Boot
updated: 2026-04-27

→ Execute plan φ docs/superpowers/plans/2026-04-27-view-invariants-and-goldens.md (closes fo-abv) — start with `bd show fo-abv` then Task 0.

✓ done
- Filed fo-abv (leaderboard rows not aggregated by label)
- Wrote + reviewed + patched the QA plan (invariants + pipeline goldens)

‡ traps
- Plan reuses `-update` flag from view_test.go:18 — ✗ declare a second one
- view_test.go already owns TestMain — build fo binary lazily via sync.Once

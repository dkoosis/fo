# Boot
updated: 2026-06-15

→ next: `bd ready`, pick. No live bugs left — remaining are P3 design refactors (29w/061 extract-or-leave; e0v/ajw decompose, plan first) + `human`-labeled calls.

✓ done
- Verified May bug audit (#257-271) fully resolved; fixed memory note.
- Hardened #269 (sarif depth-bomb guard, tested); #268 needs no code (temp+rename neutralizes it).

‡ traps
- 8 test files uncommitted (6 prior + sarif reader pair) — commit unauthorized.
- doc-governance hook blocks root *.md.

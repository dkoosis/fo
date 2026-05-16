# Boot
updated: 2026-05-16

→ review/merge PRs in order: #275 (tidy) → #272 (suppress parser) → #273 (scene parser) → #274 (cluster heuristic). Run `gh pr list`.

✓ done
- shipped 4 PRs in one drain: fo-m97, fo-u15.2.1, fo-fl0.1, fo-u15.3.1
- decomposed fo-fl0 epic into 5 child beads with deps wired

‡ traps
- subagent worktrees may not persist files saved to project paths — verify with `ls` after agent reports "saved to X"

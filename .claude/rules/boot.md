# Boot
updated: 2026-04-26

→ `bd ready` — pick from unblocked v2 children. fo-7f5.4 + fo-lxf shipped but in_progress pending dk review; `bd close` them after eyeball.

✓ done
- fo-7f5.4: pkg/view (8 variants, mono goldens + color smokes)
- fo-lxf: testdata/golden/v1 (11 fixtures × 3 formats)

‡ traps
- pre-commit doc-governance hook is local-only (.git/hooks/, untracked); .md allowlist fix won't survive fresh clone

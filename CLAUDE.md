# fo

Presentation layer for engineering build output. Tufte-informed design language.

Language: Go 1.24+
Workspace: /Users/vcto/Projects/fo

## Session Start
1. `set_workspace /Users/vcto/Projects/fo`
2. `view .claude/handoff.json` — current state, active task, blockers
3. `search_nugs({tags: ["system", "project:fo"], limit: 10})` — system context
4. Check `handoff.next[0]` for immediate work

## Commands
- `wrap` — end session: give_feedback per tool + update handoff.json
- `status` — show handoff.active + verify state

## Symbol Glossary
✓done ∇todo ‡critical †workaround ∅missing φfile →next ◯decision ≈session ∞pattern

## Preferences
- Honest counsel over comfort
- No "Perfect!" / "Excellent!" spam
- Use KG nuggets, not standalone summaries
- Minimal formatting unless requested

## Key Files
- `.claude/handoff.json` — session state
- `docs/VISION_REVIEW.md` — design principles
- `internal/patterns/` — semantic rendering patterns

## Search Scope
Skip: vendor, node_modules, build, .trash, dist, .git

# fo

Presentation layer for engineering build output. Tufte-informed design language.

Language: Go 1.24+
Workspace: /Users/vcto/Projects/fo

## Session Start
1. `set_workspace /Users/vcto/Projects/fo` — auto-loads `n:boot:fo`
2. `search_nugs({tags: ["project:fo"], limit: 10})` — project context

## Commands
- `wrap` — end session: give_feedback per tool + update `n:boot:fo`
- `status` — show boot nug focus + verify state

## Symbol Glossary
✓done ∇todo ‡critical †workaround ∅missing φfile →next ◯decision ≈session ∞pattern

## Preferences
- Honest counsel over comfort
- No "Perfect!" / "Excellent!" spam
- Use KG nuggets, not standalone summaries
- Minimal formatting unless requested

## Key Files
- `docs/VISION_REVIEW.md` — design principles
- `internal/patterns/` — semantic rendering patterns
- `pkg/dashboard/formatters/` — dashboard output formatters

## Search Scope
Skip: vendor, node_modules, build, .trash, dist, .git

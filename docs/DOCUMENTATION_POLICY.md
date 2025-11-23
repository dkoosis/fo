# Documentation Policy

**Status**: Active
**Last Updated**: 2025-11-23
**Enforcement**: Pre-commit hook

## Core Principle

**Document deliberately. Consolidate aggressively. Never create a new file when you can update an existing one.**

---

## Allowed Documentation Locations

### ✅ Module Documentation
**Location**: `internal/[module]/README.md` or `[package]/README.md`
**Rule**: ONE README.md per module/package.

**Purpose**:
- Module overview and purpose
- Public API documentation
- Architecture and design decisions specific to this module
- Usage examples
- Testing approach

**Why README.md:**
- Universal convention - everyone knows to look here
- GitHub auto-renders when browsing directories
- Tool support (godoc, doc generators, IDE preview)
- Zero learning curve for new contributors

**Never create**:
- `NOTES.md`, `TODO.md`, `ARCHITECTURE.md` (consolidate into README.md)
- `module.md`, `[modulename].md` (use README.md)
- ALL_CAPS temporary files
- Multiple .md files in same module directory

### ✅ Binary Documentation
**Location**: `cmd/README.md`
**Rule**: ONE README.md for the binary.

**Purpose**:
- Command-line usage
- Configuration options
- Installation instructions
- Examples

### ✅ Architecture Decision Records (ADRs)
**Location**: `docs/adr/ADR-NNN-title.md`
**Rule**: Numbered sequentially (ADR-001, ADR-002...), immutable after acceptance.

**Format**:
```
ADR-001-pattern-based-architecture.md
ADR-002-use-mage-for-builds.md
```

**When to create an ADR**:
- Significant architectural choices
- Technology selections
- Cross-cutting design decisions
- Changes that affect multiple modules

**When NOT to create an ADR**:
- Implementation details (use README.md)
- Minor refactorings (use decisions.log)
- Temporary experiments (use decisions.log)

### ✅ Core Top-Level Documentation
**Location**: `docs/`
**Rule**: Only for major cross-cutting concerns.

**Allowed files**:
- `PATTERNS.md` - Pattern examples and usage
- `MIGRATION.md` - Migration guides
- `VISION_REVIEW.md` - Project vision and goals
- `README.md` - Documentation index
- `DOCUMENTATION_POLICY.md` - This file

**Threshold**: If it doesn't affect 3+ modules, it doesn't belong here.

### ✅ Daily Decisions Log
**Location**: `decisions.log` (root)
**Rule**: Rolling log for minor-to-medium implementation changes.

**Format**:
```
2025-11-23: Created ADR directory structure
2025-11-23: Adopted pattern-based architecture for mageconsole
```

**When to use**:
- Implementation choices that don't warrant an ADR
- Refactoring rationale
- Bug fix explanations
- Performance tuning notes

---

## Quick Reference for LLM Agents

### Absolute Rules

1. **Module docs** live at `[module]/README.md` — one per module.
2. **Binary docs** live at `cmd/README.md` — one for the binary.
3. **Architectural decisions** use numbered ADRs in `docs/adr/`.
4. **Core cross-cutting docs** stay within `docs/{PATTERNS,MIGRATION,README,DOCUMENTATION_POLICY}.md`.
5. **Minor implementation notes** go to the root `decisions.log`.

### Never Do This

- Create `NOTES.md`, `TODO.md`, `ARCHITECTURE.md`, or any ALL_CAPS scratch pad in module directories.
- Drop `docs/[feature].md` files—move the content into the owning module README or an ADR.
- Add root-level markdown files beyond `README.md` and `decisions.log`.

### Before Creating Markdown

Ask, in order:

1. Can this update an existing module README? → **Yes**: edit the README.
2. Does it document a lasting architecture decision? → **Yes**: write an ADR.
3. Is it a small operational note? → **Yes**: log it in `decisions.log`.
4. Still unsure? Default to updating an existing README.

### Session Checklist for Assistants

- Update README files instead of creating ad-hoc markdown.
- Challenge requests to "just document" by proposing README or ADR updates.
- Avoid creating markdown without validating an approved location.
- Remember: every new markdown file is **wrong until proven right**.

---

## ❌ Forbidden Patterns

### Never Create
- `[module]/NOTES.md` → Consolidate into README.md
- `[module]/TODO.md` → Use GitHub issues
- `[module]/ARCHITECTURE.md` → Part of README.md
- `[module]/module.md` → Use README.md
- `docs/[random-feature].md` → Use module README.md or ADR
- ALL_CAPS_TEMPORARY_FILES.md → Use decisions.log
- Root-level .md files (except README.md, decisions.log)

### Anti-Patterns
- **"I'll just create a quick note file"** → Update README.md or use decisions.log
- **"This needs its own doc"** → Ask: Can it be a section in README.md?
- **"Temporary file for tracking"** → Use GitHub issues or decisions.log
- **"Draft documentation"** → Draft in PR description, finalize in proper location
- **"README.md is getting long"** → That's fine! Use headings and TOC

---

## Decision Tree

```
New documentation needed?
│
├─ Module-specific?
│  └─ YES → Update [module]/README.md
│
├─ Binary usage docs?
│  └─ YES → Update cmd/README.md
│
├─ Architectural decision?
│  └─ YES → Create docs/adr/ADR-NNN-title.md
│
├─ Minor implementation note?
│  └─ YES → Append to decisions.log
│
├─ Affects 3+ modules?
│  └─ YES → Justify top-level docs/ file
│  └─ NO  → Belongs in module README.md
│
└─ Still unsure?
   └─ Don't create a new file. Update README.md or use decisions.log.
```

---

## Enforcement

### Pre-commit Hook
Located at `.git/hooks/pre-commit`

**Behavior**:
- Blocks new .md files outside allowed locations
- Provides helpful prompts and suggestions
- Requires `--no-verify` to override (with justification)

**Installation**: Automatic in git repository.

### Code Review
Reviewers MUST:
- Challenge new .md file creation
- Verify proper location
- Suggest consolidation opportunities
- Reject PRs with orphaned docs

---

## Examples

### ❌ Bad: Creating New Files
```bash
# Someone adds new feature
touch mageconsole/PATTERNS.md              # NO! Use README.md
touch docs/mageconsole-design.md          # NO! Use module README or ADR
touch CONSOLE_NOTES.md                    # NO! Use decisions.log
```

### ✅ Good: Updating Existing
```bash
# Add section to existing README.md
vim mageconsole/README.md
# Add "## Pattern Examples" section

# Or create ADR if architectural
vim docs/adr/ADR-002-use-mage-for-builds.md

# Or log minor decision
echo "2025-11-23: Switched console renderer to use patterns" >> decisions.log
```

---

## Rationale

### Why This Policy Exists
1. **Cognitive Load**: Finding docs should be instant, not a scavenger hunt
2. **Maintenance Burden**: Every file needs updating, reviewing, refactoring
3. **Knowledge Loss**: Valuable insights get buried in sprawl
4. **Decision Fatigue**: "Where should this go?" shouldn't be hard

### Philosophy
- **Consolidation over proliferation**
- **Convention over decision**
- **Discoverability over organization**
- **One place to look, not twelve**

---

## FAQ

**Q: What if my README.md gets too long?**
A: That's okay! Long is better than scattered. Use headings and a table of contents. GitHub auto-renders TOC for long files.

**Q: But I'll have 20 tabs all named "README.md"!**
A: Configure your editor to show paths in tabs. Most modern editors support this. VSCode: `"workbench.editor.labelFormat": "medium"`

**Q: What about diagrams?**
A: Embed mermaid diagrams in .md files. GitHub renders these beautifully.

**Q: What if I really need a new top-level doc?**
A: Justify it in PR description. Explain why it can't be module README.md or ADR. Get approval.

**Q: Can I override the pre-commit hook?**
A: Yes, with `--no-verify`. But you MUST justify in commit message why.

---

## Changelog

- **2025-11-23**: Adapted from orca project documentation policy
  - Established allowed locations for fo project
  - Created ADR directory structure
  - Introduced decisions.log pattern
  - Simplified for smaller project scope

---

## See Also

- `docs/adr/README.md` - ADR template and guidelines
- `.git/hooks/pre-commit` - Enforcement hook

# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the `fo` project.

## Format

ADRs are numbered sequentially and follow this naming convention:
```
ADR-NNN-title-in-kebab-case.md
```

Examples:
- `ADR-001-pattern-based-architecture.md`
- `ADR-002-use-mage-for-builds.md`

## What Should Be an ADR?

Create an ADR for:
- Significant architectural choices
- Technology selections
- Cross-cutting design decisions
- Changes that affect multiple modules
- Patterns that should be followed consistently

Do NOT create an ADR for:
- Implementation details (use module README.md)
- Minor refactorings (use decisions.log)
- Temporary experiments (use decisions.log)

## Template

```markdown
# ADR-NNN: Title

**Status**: Draft | Proposed | Accepted | Deprecated | Superseded by ADR-XXX
**Date**: YYYY-MM-DD
**Deciders**: [List key people involved]

## Context

What is the issue we're facing? What factors are driving this decision?

## Decision

What did we decide to do?

## Consequences

What are the positive and negative outcomes of this decision?

### Positive
- Benefit 1
- Benefit 2

### Negative
- Trade-off 1
- Trade-off 2

## Alternatives Considered

What other options did we evaluate and why did we reject them?

## References

- Links to relevant docs, discussions, or resources
```

## Numbering

Use the next available number. Check existing ADRs to find the highest number, then increment.

ADRs are **immutable** after acceptance. If you need to change a decision, create a new ADR that supersedes the old one.

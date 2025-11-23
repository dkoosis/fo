# ADR-001: Pattern-Based Architecture for mageconsole

**Status**: Accepted
**Date**: 2025-11-23
**Deciders**: fo project team

## Context

The `fo` project needed a clean, maintainable way to generate formatted output from command execution results. Traditional approaches using string concatenation or template systems become unwieldy and hard to test as output formats grow in complexity.

## Decision

Implement a pattern-based architecture for the `mageconsole` package where:

1. **Patterns** are composable, reusable formatters that know how to render specific types of data
2. **Renderers** use patterns to generate formatted output
3. **Console** coordinates execution and rendering with a clean API

This architecture is inspired by the Command pattern and Strategy pattern, providing:
- Type-safe formatting through Go interfaces
- Easy testing of individual formatters
- Composition of complex outputs from simple parts
- Clear separation between execution and presentation

## Consequences

### Positive

- **Testability**: Each pattern can be tested in isolation
- **Reusability**: Patterns can be shared across different renderers
- **Maintainability**: Adding new output formats doesn't require modifying existing code
- **Composability**: Complex outputs built from simple, well-tested components
- **Type Safety**: Compile-time guarantees about formatter compatibility

### Negative

- **Initial Learning Curve**: Team members need to understand the pattern system
- **More Abstraction**: Simple outputs require more code than direct string formatting
- **Pattern Proliferation**: Risk of creating too many specialized patterns

## Alternatives Considered

### Direct String Concatenation
- **Rejected**: Becomes unmaintainable as complexity grows
- Hard to test individual components
- Difficult to change output formats

### Template-Based System (text/template)
- **Rejected**: Runtime errors instead of compile-time safety
- Harder to compose and reuse formatting logic
- Less IDE support and refactoring safety

### Builder Pattern Only
- **Considered**: Simpler but less flexible
- Doesn't provide the same level of reusability
- Mixing execution and formatting concerns

## References

- See `mageconsole/` package for implementation
- See `docs/PATTERNS.md` for pattern examples
- Command Pattern: https://en.wikipedia.org/wiki/Command_pattern
- Strategy Pattern: https://en.wikipedia.org/wiki/Strategy_pattern

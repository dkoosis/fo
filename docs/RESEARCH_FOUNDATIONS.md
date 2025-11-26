# Research Foundations

This document provides citations and references for the research-based design decisions in fo. The design system draws from multiple fields: cognitive load theory, information visualization, and human-computer interaction.

## Cognitive Load Theory

### Sweller's Cognitive Load Theory

**Reference**: Sweller, J. (1988). "Cognitive load during problem solving: Effects on learning." *Cognitive Science*, 12(2), 257-285.

**Application in fo**:
- **Cognitive load heuristics** (`pkg/design/recognition.go:413-421`): The system automatically simplifies output when cognitive load is high (many errors, large output).
- **Load thresholds**: Error count > 5 or output size > 100 lines triggers high cognitive load mode.
- **Adaptive rendering**: Output complexity adapts based on estimated cognitive load.

**Code Reference**:
```go
// pkg/design/recognition.go:413-421
// Research-based heuristics (Zhou et al., Sweller)
// High load: >5 errors or >100 lines → simplify rendering
// Medium load: >0 errors or >3 warnings or >30 lines
// Low load: otherwise
```

### Zhou et al. Study on Developer Attention

**Reference**: Zhou, J., Walker, R. J., & Kafura, D. (2019). "Characterizing and identifying error-prone classes using program analysis metrics." *Empirical Software Engineering*, 24(4), 2057-2093.

**Note**: While the code references "Zhou et al." for error log attention patterns, the specific thresholds (5 errors, 3 warnings) are heuristics derived from general cognitive load principles rather than direct citations from this paper. The paper focuses on error-prone code patterns, but the principle of reducing cognitive load when error density is high is consistent with Sweller's theory.

**Application in fo**:
- **Error threshold**: When error count exceeds 5, the system switches to high cognitive load mode.
- **Warning threshold**: When warning count exceeds 3, the system switches to medium cognitive load mode.
- **Output size threshold**: When output exceeds 100 lines, the system assumes high cognitive load.

## Information Visualization

### Tufte's Principles

**Reference**: Tufte, Edward R. (1983). *The Visual Display of Quantitative Information*. Graphics Press.

**Application in fo**:
- **Data-ink ratio**: DensityMode types maximize information per line.
- **Sparklines**: Word-sized graphics using Unicode blocks.
- **Small multiples**: Pattern composition enables side-by-side comparison.

See [TUFTE_PRINCIPLES.md](./TUFTE_PRINCIPLES.md) for detailed documentation.

### Heer & Bostock's Information Visualization Research

**Reference**: Heer, J., & Bostock, M. (2010). "Crowdsourcing graphical perception: Using mechanical turk to assess visualization design." *CHI '10: Proceedings of the SIGCHI Conference on Human Factors in Computing Systems*.

**Application in fo**:
- **Color perception**: Theme system uses colors that are distinguishable for colorblind users.
- **Visual hierarchy**: Patterns use indentation, spacing, and typography to create clear hierarchy.

## Human-Computer Interaction

### Norman's Design Principles

**Reference**: Norman, D. A. (2013). *The Design of Everyday Things: Revised and Expanded Edition*. Basic Books.

**Application in fo**:
- **Affordances**: Icons and symbols clearly indicate their meaning (✓ for success, ✗ for failure).
- **Feedback**: Immediate visual feedback for command execution status.
- **Mapping**: Clear relationship between command output and visual representation.

### Nielsen's Usability Heuristics

**Reference**: Nielsen, J. (1994). "Enhancing the explanatory power of usability heuristics." *CHI '94: Proceedings of the SIGCHI Conference on Human Factors in Computing Systems*.

**Application in fo**:
- **Visibility of system status**: Progress indicators and status messages.
- **Error prevention**: Clear error messages with context.
- **Recognition rather than recall**: Icons and patterns are self-explanatory.

## Pattern Recognition and Classification

### Line Classification Heuristics

The line classification system in `pkg/design/recognition.go` uses pattern matching based on common conventions:

- **Error patterns**: Lines containing "error", "failed", "fatal", exit codes
- **Warning patterns**: Lines containing "warning", "warn", "deprecated"
- **Success patterns**: Lines containing "ok", "pass", "success", "✓"
- **Progress patterns**: Lines containing percentages, progress indicators

These heuristics are based on empirical observation of common tool output formats rather than formal research, but they follow established conventions in the software development community.

## Cognitive Load Heuristics

### Thresholds and Rationale

The cognitive load thresholds in `pkg/design/recognition.go:414-420` are based on:

1. **Error threshold (5)**: Based on the observation that developers can typically track 3-7 items in working memory (Miller's Law: 7±2). Five errors represent a manageable but high cognitive load.

2. **Warning threshold (3)**: Warnings are less critical than errors, so the threshold is lower. Three warnings indicate potential issues that require attention.

3. **Output size threshold (100 lines)**: Based on screen real estate and scrolling behavior. Most terminals show 24-50 lines, so 100 lines requires scrolling, increasing cognitive load.

4. **Medium load threshold (30 lines)**: Represents approximately one screen of output, where cognitive load begins to increase.

These thresholds are heuristics derived from general principles rather than specific research studies, but they align with:
- Miller, G. A. (1956). "The magical number seven, plus or minus two: Some limits on our capacity for processing information." *Psychological Review*, 63(2), 81-97.
- Cowan, N. (2001). "The magical number 4 in short-term memory: A reconsideration of mental storage capacity." *Behavioral and Brain Sciences*, 24(1), 87-114.

## Future Research Integration

### Areas for Further Research

1. **Empirical validation**: Conduct user studies to validate cognitive load thresholds.
2. **Eye-tracking studies**: Measure where developers look in build output to optimize layout.
3. **A/B testing**: Compare different density modes and layouts for effectiveness.
4. **Accessibility research**: Validate color choices and patterns for colorblind users.

### Potential Citations to Add

- **Color accessibility**: Stone, M. (2006). "Color and usability." *ACM Interactions*, 13(4), 48-49.
- **Terminal readability**: Bargas-Avila, J. A., & Obermeier, K. (2013). "User experience with command-line interfaces." *CHI '13: Extended Abstracts on Human Factors in Computing Systems*.
- **Information foraging**: Pirolli, P., & Card, S. (1999). "Information foraging." *Psychological Review*, 106(4), 643-675.

## Code Annotations

When adding new cognitive load heuristics or visualization features, please:

1. **Cite sources**: Add comments referencing the research that informed the decision.
2. **Document rationale**: Explain why specific thresholds or approaches were chosen.
3. **Note limitations**: If heuristics are based on general principles rather than specific studies, note this.

**Example**:
```go
// Cognitive load threshold based on Miller's Law (7±2 items in working memory).
// Five errors represent high cognitive load, triggering simplified rendering.
// Reference: Miller, G. A. (1956). "The magical number seven, plus or minus two."
const HighLoadErrorThreshold = 5
```

## Related Documentation

- [Tufte Principles](./TUFTE_PRINCIPLES.md) - Design principles from information visualization
- [Architecture](./design/architecture.md) - System design overview
- [Pattern Types](./design/pattern-types.md) - Pattern documentation


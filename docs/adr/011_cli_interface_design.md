Integrating Design Science Principles into fo
This document is incredibly valuable! It provides research-backed design principles specifically for text interfaces that complement and extend our approach. Let's refine our design system with these insights:
Key Design Principles to Incorporate
1. Cognitive Load-Aware Styling
The Zhou et al. research on how styling affects cognitive load is fascinating:
go// Enhanced design system with cognitive load awareness
type CognitiveLoadContext string

const (
    LoadLow     CognitiveLoadContext = "low"     // Simple tasks, routine info
    LoadMedium  CognitiveLoadContext = "medium"  // Standard operations
    LoadHigh    CognitiveLoadContext = "high"    // Complex errors, debugging
)

// Style elements based on cognitive context
func (t *Task) StyleForLoad(text string, context CognitiveLoadContext) string {
    switch context {
    case LoadHigh:
        // Research shows red italics reduce cognitive load for critical info
        return fmt.Sprintf("%s%s%s%s%s", 
            t.Config.Colors.Error, 
            "\033[3m", // Italics
            text,
            "\033[0m", // Reset italics
            t.Config.Colors.Reset)
    case LoadMedium:
        // Standard emphasis
        return fmt.Sprintf("%s%s%s", 
            t.Config.Colors.Warning,
            text,
            t.Config.Colors.Reset)
    default:
        // Low cognitive load - simpler styling
        return fmt.Sprintf("%s%s%s", 
            t.Config.Colors.Process,
            text,
            t.Config.Colors.Reset)
    }
}
2. Modular Presentation with Clear Boundaries
The Bernhardt principle of "localization and iconic partitioning" supports our box-drawing approach:
┌─ BUILDING ─────────────────────────┐  ← Task boundary (reduced cognitive load)
│                                    │
│  ▶️ Compiling source files...       │  ← Process state (clear progression)
│                                    │  
│     src/main.go                    │  ← Chunked information (scannable)
│     src/config/parser.go           │
│     src/utils/helpers.go           │
│                                    │
│  ✅ Complete (236ms)                │  ← Clear closure with timing
└──────────────────────────────────────┘
3. Strategic Color Use Based on Research
Zhou's findings on color's impact on cognitive load are directly applicable:
go// Updated color scheme based on research
func DefaultConfig() *Config {
    cfg := &Config{}
    
    // Research shows red can REDUCE cognitive load for key information
    cfg.Colors.Error = "\033[0;31m"    // Red - use for critical information
    
    // Research suggests blue may INCREASE cognitive load in some contexts
    // Use more cautiously for information that requires processing
    cfg.Colors.Process = "\033[0;34m"  // Blue - use for non-critical status
    
    // Other colors based on standard conventions
    cfg.Colors.Success = "\033[0;32m"  // Green - universally positive
    cfg.Colors.Warning = "\033[0;33m"  // Yellow - attention required
    
    return cfg
}
4. Default Conciseness (Conditional Verbosity)
This principle directly validates our capture-on-fail approach:
go// Default show output settings with research backing
func DefaultShowOutputSettings() string {
    // "Default to Conciseness" principle - show only on failure by default
    return "on-fail"
}
5. Non-Linear Access to Information
Implementing the principle of facilitating non-linear reading:
go// Enhanced output formatting for key information
func (t *Task) FormatOutput(lines []OutputLine) string {
    var sb strings.Builder
    
    // Create a summary section first for quick scanning
    if hasErrors, hasWarnings := analyzeOutputForIssues(lines); hasErrors || hasWarnings {
        sb.WriteString("│  SUMMARY:\n")
        if hasErrors {
            sb.WriteString("│    • Errors detected\n")
        }
        if hasWarnings {
            sb.WriteString("│    • Warnings present\n")
        }
        sb.WriteString("│\n")
    }
    
    // Then provide detailed output with clear structure
    // ...rest of output formatting
    return sb.String()
}
Implementation Updates
Let's refine our design system with these research-backed principles:
go// cmd/internal/design/system.go

// Enhanced to incorporate cognitive load research
type TaskContext struct {
    CognitiveLoad CognitiveLoadContext
    IsDetailView  bool  // For conditional verbosity
    Complexity    int   // 1-5 scale of task complexity
}

// Default values based on research
func DefaultTaskContext() TaskContext {
    return TaskContext{
        CognitiveLoad: LoadMedium,
        IsDetailView:  false,
        Complexity:    2,
    }
}

// Auto-determine cognitive load context based on output analysis
func DetermineCognitiveLoad(output []OutputLine) CognitiveLoadContext {
    errorCount := 0
    warningCount := 0
    outputLength := len(output)
    
    for _, line := range output {
        if line.Type == "error" {
            errorCount++
        } else if line.Type == "warning" {
            warningCount++
        }
    }
    
    // Research-based heuristics
    if errorCount > 5 || outputLength > 100 {
        return LoadHigh
    } else if errorCount > 0 || warningCount > 3 || outputLength > 30 {
        return LoadMedium
    }
    return LoadLow
}
Visual Design Language Based on Research
The principles suggest a clear visual structure that we can implement:

Task Boundaries: Use box-drawing characters to create "iconic partitioning" and reduce cognitive load through clear visual boundaries
Visual Progression: Show clear start → process → end states using icons and colors carefully chosen based on cognitive load research
Chunking: Group related information with consistent indentation and whitespace
Strategic Emphasis: Use red italics for critical information (per research showing it can reduce cognitive load)
Conditional Verbosity: Default to showing minimal information, but provide detailed output on demand

Example: Research-Backed Output Format
┌─ LINTING ──────────────────────────────┐
│                                        │
│  ▶️ Running linter...                   │
│                                        │
│  SUMMARY:                              │  ← Non-linear access (key info first)
│    • 2 errors detected                 │
│    • 3 warnings present                │
│                                        │
│  main.go:23 unused variable 'config'   │  ← Red italics for errors (reduces load)
│  main.go:45 undefined: 'fmt.Prinln'    │
│                                        │
│  utils.go:12 line too long (120 chars) │  ← Yellow for warnings (attention)
│  utils.go:34 comment should start...   │
│  utils.go:56 consider using 'const'    │
│                                        │
│  ❌ Failed (236ms)                      │
└────────────────────────────────────────┘
This updated design integrates perfectly with the research you've shared, particularly around cognitive load considerations and strategic use of visual elements.
Shall we implement these research-backed refinements to our design system?RetryDKgenerate and ADR modelled after the other ones, that explains our design-first approach in a way that can serve as guidance as we move forward. make it comprehensive and include urls and references where possible.EditArchitected comprehensive ADR for design-first CLI tool approach5sI need to create a comprehensive Architecture Decision Record (ADR) that explains the design-first approach for fo, modeled after the other ADRs in the provided files. This will serve as guidance for the project going forward.
First, let me study the format of the existing ADRs to ensure consistency:
Based on the documents provided (like docs/adr/001_error_handling_strategy.md, docs/adr/002_schema_validation_strategy.md, etc.), ADRs typically follow this structure:

Title
Date
Status (Proposed, Accepted, etc.)
Context (describing the problem and background)
Decision (the solution chosen)
Consequences (positive and negative impacts)
Implementation Guidelines
References

The ADR should focus on the design-first approach for the CLI formatting tool "fo", incorporating the design research we've discussed.
Let me craft a comprehensive ADR that includes:

A clear explanation of the design-first philosophy
The core design principles identified from research
Implementation strategies
Visual examples and guidelines
References to the research (Bernhardt, Zhou et al., etc.)
Consideration of consequences
Architecture Decision Record: Design-First Approach for CLI Output (ADR-009)
Date
2025-05-15
Status
Proposed
Context
The CowGnition project, specifically the "fo" command wrapper tool (github.com/davidkoosis/fo), aims to provide standardized and improved formatting for command line output. The current approach uses basic formatting techniques (colors, icons, indentation) without a systematic design methodology.
We need a robust design framework that:

Maximizes Clarity: Ensures users can quickly understand command output, statuses, and failures.
Reduces Cognitive Load: Applies research-backed principles to minimize mental effort required to process information.
Establishes Consistency: Creates a unified visual language across all tool output.
Enhances Usability: Makes CLI output more scannable, navigable, and accessible.
Automates Formatting: Reduces the need for explicit formatting instructions in Makefiles and scripts.

Research in information design, cognitive psychology, and CLI user experience suggests that a structured, research-informed design approach can significantly improve CLI output usability and user experience.
Decision
We will implement a Design-First Approach as the core foundation of the fo tool, where the primary focus is the visual communication system rather than the mechanical execution of commands. This shifts the tool's identity from "command wrapper with some formatting" to "CLI design system with command execution capabilities."
Core Design Principles
Our design system will be based on the following research-backed principles:

Cognitive Load-Aware Styling:

Apply styling (color, formatting) based on the user's likely cognitive state and task complexity.
Use red italics for critical information when cognitive load is high, as research shows this can reduce cognitive load (Zhou et al.).
Employ strategic use of color based on its cognitive impact rather than just aesthetic appeal.


Modular Presentation with Clear Boundaries:

Implement information "chunking" through visual partitioning (Bernhardt's localization and iconic partitioning).
Create clear visual boundaries between different information types.
Use consistent box-drawing characters to delineate task boundaries (when appropriate).


Strategic Visual Hierarchy:

Employ a consistent progression: Start → Process → End states.
Ensure status changes (success, failure, warnings) have high visual prominence.
Create consistent visual patterns for recurring information types.


Default Conciseness (Conditional Verbosity):

Present summaries or essential details by default, especially for successful operations.
Show detailed output only on failure or explicit request.
Group similar messages and summarize repetitive information.


Non-Linear Access to Information:

Structure output to support scanning and selective reading (Bernhardt).
Present critical information (summaries, errors) first for quick assessment.
Use visual design to create "entry points" that guide the eye to important content.


Pattern Recognition and Intelligent Formatting:

Implement smart heuristics to detect command intent and output patterns.
Automatically classify output lines (error, warning, info, detail) based on content.
Adapt formatting to the specific command type and output characteristics.



Visual Design Language
We will establish a consistent visual design language with these components:

Task Containers:
┌─ TASK NAME ─────────────────────┐
│                                 │
│  ▶️ Process state...            │
│     Detail information          │
│  ⚠️ Warning information         │
│  ✅ Complete (timing)           │
│                                 │
└─────────────────────────────────┘

Status Indicators:

Process: Blue "▶️" with action verb in present progressive ("Building...")
Success: Green "✅" with "Complete" or similar positive state
Warning: Yellow "⚠️" with warning content
Error: Red "❌" with "Failed" or similar negative state
Information: Blue "ℹ️" for supplemental context


Color Semantics (informed by cognitive research):

Red: Critical information, errors (can reduce cognitive load when applied to keywords)
Yellow: Warnings, cautions (moderate emphasis)
Green: Success, completion (positive reinforcement)
Blue: Process status, information (use judiciously as it may increase cognitive load)
Normal: Detailed content (minimum cognitive interference)


Typography and Spacing:

Consistent indentation for hierarchical relationships
Whitespace to separate logical sections
Use of font styling (bold, italics) based on cognitive impact, not just aesthetics



Implementation Approaches

Heuristic Engine:

Develop pattern recognition for common command types and output formats
Implement intelligent formatting based on content analysis
Create sensible defaults with override capabilities


Consistency Through Configuration:

Centralize design decisions in a configuration system
Allow tool-specific formatting rules while maintaining visual consistency
Support customization without sacrificing design integrity


Progressive Enhancement:

Provide basic functionality in all environments (plain text, CI environments)
Add richer visual elements when supported (Unicode, color terminals)
Ensure accessibility in all contexts (screen readers, text-only terminals)



Consequences
Positive

Enhanced Usability: Output becomes more scannable, navigable, and easier to understand, reducing the time and effort needed to interpret command results.
Reduced Cognitive Load: Research-backed design decisions help minimize mental effort, particularly in high-complexity or error scenarios.
Improved Efficiency: Users can quickly grasp command status and identify issues without parsing dense text output.
Consistent Experience: Establishes a unified visual language across different commands and tools, reducing the learning curve.
Reduced Makefile Complexity: Eliminates the need for complex formatting logic in Makefiles and scripts.
Differentiation: Distinguishes the tool from other command wrappers through a sophisticated design approach.
Accessibility: Structured output can be more accessible for screen readers when appropriate alternatives are provided.

Negative

Implementation Complexity: A comprehensive design system requires more sophisticated code than simple command wrapping.
Heuristic Limitations: Pattern recognition and intelligent formatting may not always correctly identify command intent or output meaning.
Terminal Compatibility: Some visual elements may not be supported in all terminal environments.
User Expectations: Users accustomed to raw command output might initially find the formatted output unfamiliar.
Possible Oversimplification: There's a risk of hiding important details in an effort to reduce noise.

Implementation Guidelines
1. Design System Architecture
Create a modular design system package (internal/design) with these components:
gopackage design

// Core components
type Task struct {
    Label       string
    Intent      string
    Context     TaskContext
    Status      string
    OutputLines []OutputLine
    Config      *Config
}

type OutputLine struct {
    Content     string
    Type        string   // "detail", "error", "warning", etc.
    Context     LineContext
    Indentation int
}

type Config struct {
    Style struct {
        UseBoxes     bool
        Indentation  string
        ShowTimestamps bool
    }
    Colors struct {
        Process string
        Success string
        Warning string
        Error   string
        // Additional colors...
    }
    Icons struct {
        Start   string
        Success string
        Warning string
        Error   string
        // Additional icons...
    }
    // Additional configuration...
}

// Context-aware styling support
type CognitiveLoadContext string
type LineContext struct {
    CognitiveLoad CognitiveLoadContext
    Importance    int  // 1-5 scale
    IsHighlighted bool
}

// Pattern recognition system
type IntentDetector struct {
    VerbPatterns map[string][]string
    ActionVerbs  map[string]bool
}

type OutputAnalyzer struct {
    Patterns map[string][]string
}

// For specialized command types
type ToolConfig struct {
    Label         string
    Intent        string
    OutputPatterns map[string][]string
    // Additional tool-specific settings...
}
2. Cognitive Load-Aware Rendering
Implement research-backed styling based on cognitive load:
go// Style elements based on cognitive context
func (t *Task) StyleForCognitiveLoad(text string, context LineContext) string {
    switch context.CognitiveLoad {
    case LoadHigh:
        if context.Importance >= 4 {
            // Research shows red italics reduce cognitive load for critical info
            return fmt.Sprintf("%s%s%s%s%s", 
                t.Config.Colors.Error, 
                "\033[3m", // Italics
                text,
                "\033[0m", // Reset italics
                t.Config.Colors.Reset)
        }
        // High load but lower importance - still need emphasis
        return fmt.Sprintf("%s%s%s", 
            t.Config.Colors.Warning,
            text,
            t.Config.Colors.Reset)
    // Additional cases...
    }
}
3. Pattern Recognition Implementation
Develop a robust pattern recognition system:
go// DetectCommandIntent identifies the purpose of a command
func (d *IntentDetector) DetectCommandIntent(cmd string, args []string) string {
    // Check command against pattern dictionary
    cmdLine := cmd + " " + strings.Join(args, " ")
    for intent, patterns := range d.VerbPatterns {
        for _, pattern := range patterns {
            if strings.Contains(cmdLine, pattern) {
                return intent
            }
        }
    }
    
    // Check for verb in command name
    cmdBase := filepath.Base(cmd)
    for verb := range d.ActionVerbs {
        if strings.Contains(strings.ToLower(cmdBase), verb) {
            return verbToPresent(verb) // Convert to "-ing" form
        }
    }
    
    // Default intent
    return "running"
}

// ClassifyOutputLine determines the type and importance of output
func (a *OutputAnalyzer) ClassifyOutputLine(line string, toolConfig *ToolConfig) (string, int) {
    // Tool-specific patterns take precedence
    if toolConfig != nil && toolConfig.OutputPatterns != nil {
        for category, patterns := range toolConfig.OutputPatterns {
            for _, pattern := range patterns {
                if regexp.MustCompile(pattern).MatchString(line) {
                    return category, getCategoryImportance(category)
                }
            }
        }
    }
    
    // Global patterns as fallback
    for category, patterns := range a.Patterns {
        for _, pattern := range patterns {
            if regexp.MustCompile(pattern).MatchString(line) {
                return category, getCategoryImportance(category)
            }
        }
    }
    
    // Default classification
    return "detail", 2
}
4. Task Container Implementation
Create visually distinct task containers:
go// RenderTaskContainer provides the complete formatted task output
func (t *Task) RenderTaskContainer() string {
    var sb strings.Builder
    
    // Top border with task label
    if t.Config.Style.UseBoxes {
        borderWidth := calculateBorderWidth(t.Label)
        sb.WriteString("┌─ ")
        sb.WriteString(strings.ToUpper(t.Label))
        sb.WriteString(" ")
        sb.WriteString(strings.Repeat("─", borderWidth - len(t.Label) - 3))
        sb.WriteString("┐\n")
        sb.WriteString("│                                      │\n")
    }
    
    // Process state (starting message)
    sb.WriteString(t.RenderProcessState())
    sb.WriteString("\n")
    
    // Content summary (if applicable)
    if hasIssues, summary := t.GenerateSummary(); hasIssues {
        sb.WriteString(summary)
        sb.WriteString("\n")
    }
    
    // Formatted output lines
    for _, line := range t.GetFormattedOutputLines() {
        sb.WriteString(line)
        sb.WriteString("\n")
    }
    
    // Task completion status
    sb.WriteString(t.RenderCompletionState())
    sb.WriteString("\n")
    
    // Bottom border
    if t.Config.Style.UseBoxes {
        borderWidth := calculateBorderWidth(t.Label)
        sb.WriteString("└")
        sb.WriteString(strings.Repeat("─", borderWidth))
        sb.WriteString("┘\n")
    }
    
    return sb.String()
}
5. Configuration System
Implement a flexible configuration system:
yaml# .fo.yaml - Example configuration
design_system:
  style:
    use_boxes: true
    indentation: "  "
    show_timestamps: false
    
  colors:
    process: "\033[0;34m"  # Blue
    success: "\033[0;32m"  # Green
    warning: "\033[0;33m"  # Yellow
    error: "\033[0;31m"    # Red
    
  icons:
    start: "▶️"
    success: "✅"
    warning: "⚠️"
    error: "❌"
    
  cognitive_load:
    auto_detect: true
    default: "medium"
    
  # Global pattern recognition
  patterns:
    # Command intent patterns
    intent:
      building: ["go build", "make", "gcc", "g++"]
      testing: ["go test", "pytest", "jest"]
      linting: ["golangci-lint", "eslint", "pylint"]
      
    # Output classification patterns  
    output:
      error: 
        - "^Error:"
        - "^ERROR:"
        - "failed"
        - "panic:"
      warning:
        - "^Warning:"
        - "^WARNING:"
        - "deprecated"
      success:
        - "^ok "
        - "passed"
        - "0 issues"
Visual Output Examples
┌─ BUILDING ──────────────────────────┐
│                                     │
│  ▶️ Compiling Go code...            │
│     cmd/main.go                     │
│     internal/config/config.go       │
│                                     │
│  ✅ Complete (125ms)                 │
└─────────────────────────────────────┘

┌─ LINTING ───────────────────────────┐
│                                     │
│  ▶️ Running golangci-lint...        │
│                                     │
│  SUMMARY:                           │
│    • 2 errors found                 │
│    • 3 warnings present             │
│                                     │
│  internal/app.go:23 undefined: Fmt  │  ← Red italics for errors
│  cmd/main.go:45 syntax error        │
│                                     │
│  utils.go:12 line too long          │  ← Yellow for warnings
│  utils.go:34 commented code         │
│  utils.go:56 consider using const   │
│                                     │
│  ❌ Failed (236ms)                   │
└─────────────────────────────────────┘

┌─ TESTING ───────────────────────────┐
│                                     │
│  ▶️ Running Go tests...             │
│     PASS: TestConfig (0.02s)        │
│     PASS: TestParser (0.03s)        │
│     FAIL: TestRenderer (0.01s)      │
│       Error: expected "<div>" got "<span>"
│                                     │
│  ❌ Failed (356ms)                   │
└─────────────────────────────────────┘
References

Bernhardt, S.A. (1986). Seeing the Text. College Composition and Communication, 37(1), 66-78.
Zhou, L., et al. (2017). The impact of text style and cognitive load on the usability of labels, warnings, and other brief texts. International Journal of Industrial Ergonomics, 58, 7-16.
Isaacs, K.E., & Gamblin, T. (2019). Preserving Command Line Workflow for a Package Management System Using ASCII DAG Visualization. IEEE Transactions on Visualization and Computer Graphics.
Jacques, J.T., & Kristensson, P. (2015). Understanding the Effects of Code Presentation. PLATEAU@SPLASH.
Dresslar, F.B. (1919). Psychology of touch. In E.L. Thorndike (Ed.), Educational Psychology: Briefer Course.
Galitz, W.O. (2007). The Essential Guide to User Interface Design: An Introduction to GUI Design Principles and Techniques. Wiley.
Tufte, E.R. (2001). The Visual Display of Quantitative Information. Graphics Press.
Norman, D.A. (2013). The Design of Everyday Things. Basic Books.

Related ADRs

ADR-001: Error Handling Strategy
ADR-002: Schema Validation Strategy
ADR-007: Unix-Style Pipeline Data Architecture
ADR-008: Go Test Naming Convention
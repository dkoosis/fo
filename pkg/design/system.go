// Package design implements a research-backed design system for CLI output.
//
// Research foundations:
//   - Cognitive Load Theory (Sweller, 1988): Adaptive rendering based on cognitive load
//   - Tufte's Information Visualization (1983): Data-ink ratio, sparklines, small multiples
//   - Miller's Law (1956): Working memory limits inform error threshold heuristics
//
// See docs/RESEARCH_FOUNDATIONS.md and docs/TUFTE_PRINCIPLES.md for detailed citations.
package design

import (
	"sync" // Import sync package
	"time"
)

// Task represents a command execution as a visual task with formatted output.
type Task struct {
	// Core properties
	Label     string
	Intent    string
	Command   string
	Args      []string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	ExitCode  int
	Status    string // "running", "success", "warning", "error"

	// Output content
	OutputLines []OutputLine
	outputLock  sync.Mutex // Mutex to protect concurrent access to OutputLines and related context

	// Configuration and context
	Config  *Config
	Context TaskContext
}

// OutputLine represents a classified line of command output.
type OutputLine struct {
	// Content and metadata
	Content     string
	Type        string // "detail", "error", "warning", "success", "info", "progress"
	Timestamp   time.Time
	Indentation int

	// Context for cognitive load-based formatting
	Context LineContext
}

// TaskContext holds information about the cognitive context of the task
// (e.g., complexity, user's likely cognitive load).
//
// Cognitive load assessment is based on Sweller's Cognitive Load Theory (1988),
// which suggests that information presentation should adapt to reduce cognitive
// processing demands when load is high.
type TaskContext struct {
	// CognitiveLoad determines styling based on research (e.g., simplify for high load).
	// Thresholds: High (>5 errors or >100 lines), Medium (>0 errors or >3 warnings or >30 lines), Low (otherwise).
	// See pkg/design/recognition.go:estimateCognitiveLoad() for implementation.
	CognitiveLoad CognitiveLoadContext

	// IsDetailView indicates if a detailed view is active, affecting verbosity.
	IsDetailView bool
	// Complexity is a heuristic (1-5) of the task's output or nature.
	Complexity int
}

// LineContext holds information about the context of an individual output line
// used for fine-grained styling decisions.
type LineContext struct {
	// CognitiveLoad at the point this line is processed/displayed.
	CognitiveLoad CognitiveLoadContext
	// Importance rating (1-5) for prioritization in display or summary.
	Importance int
	// IsHighlighted indicates if the line should receive special emphasis.
	IsHighlighted bool
	// IsSummary indicates if this line is part of a generated summary.
	IsSummary bool
	// IsInternal indicates if this error originated from fo itself (not the wrapped command).
	IsInternal bool
}

// CognitiveLoadContext represents the user's likely cognitive state when processing information.
type CognitiveLoadContext string

const (
	LoadLow    CognitiveLoadContext = "low"    // Simple, routine information.
	LoadMedium CognitiveLoadContext = "medium" // Standard operational information.
	LoadHigh   CognitiveLoadContext = "high"   // Complex errors, dense information requiring focus.
)

// LineType constants for consistent classification of output lines.
const (
	TypeDetail   = "detail"   // Default for unclassified lines.
	TypeError    = "error"    // Error messages.
	TypeWarning  = "warning"  // Warning messages.
	TypeSuccess  = "success"  // Success indicators.
	TypeInfo     = "info"     // Informational messages (e.g., from stderr not being errors).
	TypeProgress = "progress" // Progress updates.
	TypeSummary  = "summary"  // Lines that are part of a generated summary.
)

// TaskStatus constants for consistent representation of a task's overall status.
const (
	StatusRunning = "running" // Task is currently executing.
	StatusSuccess = "success" // Task completed successfully.
	StatusWarning = "warning" // Task completed with warnings.
	StatusError   = "error"   // Task failed or completed with errors.
)

// MessageType constants for RenderDirectMessage to provide type safety and prevent typos.
const (
	MessageTypeRaw     = "raw"     // Raw output without any formatting
	MessageTypeH1      = "h1"      // Level 1 header
	MessageTypeH2      = "h2"      // Level 2 header
	MessageTypeH3      = "h3"      // Level 3 header
	MessageTypeSuccess = "success" // Success message (alias for StatusSuccess)
	MessageTypeWarning = "warning" // Warning message (alias for StatusWarning)
	MessageTypeError   = "error"   // Error message (alias for StatusError)
	MessageTypeInfo    = "info"    // Info message (alias for TypeInfo)
)

// NewTask creates and initializes a new Task.
func NewTask(label, intent, command string, args []string, config *Config) *Task {
	return &Task{
		Label:     label,
		Intent:    intent,
		Command:   command,
		Args:      args,
		StartTime: time.Now(),
		Status:    StatusRunning,
		Config:    config, // Assign the provided design configuration.
		Context: TaskContext{ // Initialize context with defaults.
			CognitiveLoad: LoadMedium,
			Complexity:    2, // Default complexity.
			IsDetailView:  false,
		},
		// outputLock is automatically initialized to its zero value (unlocked mutex).
	}
}

// AddOutputLine appends a new classified output line to the task's OutputLines.
// This method is thread-safe due to the use of outputLock.
func (t *Task) AddOutputLine(content, lineType string, context LineContext) {
	t.outputLock.Lock()         // Acquire lock before modifying shared OutputLines.
	defer t.outputLock.Unlock() // Ensure lock is released when function exits.

	t.OutputLines = append(t.OutputLines, OutputLine{
		Content:   content,
		Type:      lineType,
		Timestamp: time.Now(), // Timestamp the line addition.
		Context:   context,
	})
}

// Complete finalizes the task's status based on its exit code and output analysis.
// This should be called after all output has been processed.
func (t *Task) Complete(exitCode int) {
	// These fields are typically set once after all goroutines are done,
	// so direct assignment is safe here.
	t.EndTime = time.Now()
	t.Duration = t.EndTime.Sub(t.StartTime)
	t.ExitCode = exitCode

	// Determine final status based on exit code and any errors/warnings in output.
	// hasOutputIssues safely reads OutputLines using its internal lock.
	hasErrors, hasWarnings := t.hasOutputIssues()

	switch {
	case exitCode != 0:
		t.Status = StatusError // Non-zero exit code always means an error status.
	case hasErrors:
		t.Status = StatusError // Errors in output override a success exit code.
	case hasWarnings:
		t.Status = StatusWarning
	default:
		t.Status = StatusSuccess
	}
}

// hasOutputIssues checks the collected OutputLines for any lines classified as errors or warnings.
// This method is thread-safe for reading OutputLines.
func (t *Task) hasOutputIssues() (bool, bool) {
	t.outputLock.Lock()         // Acquire lock for reading shared OutputLines.
	defer t.outputLock.Unlock() // Release lock.

	var hasErrors, hasWarnings bool
	for _, line := range t.OutputLines {
		switch line.Type {
		case TypeError:
			hasErrors = true
		case TypeWarning:
			hasWarnings = true
			// Note: TypeInfo, TypeDetail, etc., are not considered "issues" for status determination.
		}
	}
	return hasErrors, hasWarnings
}

// UpdateTaskContext heuristically adjusts the task's cognitive load and complexity
// based on the analysis of its output lines. This method is thread-safe.
func (t *Task) UpdateTaskContext() {
	t.outputLock.Lock()         // Acquire lock for reading OutputLines and writing to t.Context.
	defer t.outputLock.Unlock() // Release lock.

	errorCount := 0
	warningCount := 0
	for _, line := range t.OutputLines {
		switch line.Type {
		case TypeError:
			errorCount++
		case TypeWarning:
			warningCount++
		}
	}

	outputSize := len(t.OutputLines) // Safely read length while holding the lock.

	// Get configurable thresholds (with sensible defaults if not set)
	veryHighThreshold := t.Config.ComplexityThresholds.VeryHigh
	if veryHighThreshold == 0 {
		veryHighThreshold = 100
	}
	highThreshold := t.Config.ComplexityThresholds.High
	if highThreshold == 0 {
		highThreshold = 50
	}
	mediumThreshold := t.Config.ComplexityThresholds.Medium
	if mediumThreshold == 0 {
		mediumThreshold = 20
	}
	errorCountHighThreshold := t.Config.ComplexityThresholds.ErrorCountHigh
	if errorCountHighThreshold == 0 {
		errorCountHighThreshold = 5
	}
	warningCountMediumThreshold := t.Config.ComplexityThresholds.WarningCountMedium
	if warningCountMediumThreshold == 0 {
		warningCountMediumThreshold = 2
	}

	// Adjust complexity based on output size using configurable thresholds.
	switch {
	case outputSize > veryHighThreshold:
		t.Context.Complexity = 5
	case outputSize > highThreshold:
		t.Context.Complexity = 4
	case outputSize > mediumThreshold:
		t.Context.Complexity = 3
	default:
		t.Context.Complexity = 2 // Default/low complexity.
	}

	// Adjust cognitive load based on errors, warnings, and complexity.
	// These heuristics can be refined based on user feedback and research.
	switch {
	case errorCount > errorCountHighThreshold || t.Context.Complexity >= 4:
		t.Context.CognitiveLoad = LoadHigh
	case errorCount > 0 || warningCount > warningCountMediumThreshold || t.Context.Complexity == 3:
		t.Context.CognitiveLoad = LoadMedium
	default:
		t.Context.CognitiveLoad = LoadLow
	}
}

// OutputLinesLock provides external access to lock the task's outputLock.
// This is used by cmd/main.go to synchronize reading of OutputLines when rendering.
func (t *Task) OutputLinesLock() {
	t.outputLock.Lock()
}

// OutputLinesUnlock provides external access to unlock the task's outputLock.
func (t *Task) OutputLinesUnlock() {
	t.outputLock.Unlock()
}

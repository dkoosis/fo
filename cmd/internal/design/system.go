// Package design implements a research-backed design system for CLI output
package design

import (
	"time"
)

// Task represents a command execution as a visual task with formatted output
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

	// Configuration and context
	Config  *Config
	Context TaskContext
}

// OutputLine represents a classified line of command output
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
type TaskContext struct {
	// Cognitive load determines styling based on research
	CognitiveLoad CognitiveLoadContext

	// Task properties affecting presentation
	IsDetailView bool // For conditional verbosity
	Complexity   int  // 1-5 scale of task complexity
}

// LineContext holds information about the context of an output line
type LineContext struct {
	// Cognitive load at this point in output
	CognitiveLoad CognitiveLoadContext

	// Importance rating (1-5) for prioritization
	Importance int

	// Special rendering flags
	IsHighlighted bool
	IsSummary     bool
}

// CognitiveLoadContext represents the user's likely cognitive state
// Based on Zhou et al. research on cognitive impact of styling
type CognitiveLoadContext string

const (
	LoadLow    CognitiveLoadContext = "low"    // Simple tasks, routine info
	LoadMedium CognitiveLoadContext = "medium" // Standard operations
	LoadHigh   CognitiveLoadContext = "high"   // Complex errors, debugging
)

// LineType constants for consistent output classification
const (
	TypeDetail   = "detail"   // Standard output
	TypeError    = "error"    // Error messages
	TypeWarning  = "warning"  // Warning messages
	TypeSuccess  = "success"  // Success indicators
	TypeInfo     = "info"     // Informational messages
	TypeProgress = "progress" // Progress indicators
	TypeSummary  = "summary"  // Summary information
)

// TaskStatus constants for consistent status representation
const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusWarning = "warning"
	StatusError   = "error"
)

// NewTask creates a new task with the given label and intent
func NewTask(label, intent string, command string, args []string, config *Config) *Task {
	return &Task{
		Label:     label,
		Intent:    intent,
		Command:   command,
		Args:      args,
		StartTime: time.Now(),
		Status:    StatusRunning,
		Config:    config,
		Context: TaskContext{
			CognitiveLoad: LoadMedium, // Default to medium load
			Complexity:    2,          // Default complexity
			IsDetailView:  false,
		},
	}
}

// AddOutputLine adds a classified output line to the task
func (t *Task) AddOutputLine(content, lineType string, context LineContext) {
	t.OutputLines = append(t.OutputLines, OutputLine{
		Content:   content,
		Type:      lineType,
		Timestamp: time.Now(),
		Context:   context,
	})
}

// Complete marks the task as complete with the given exit code
func (t *Task) Complete(exitCode int) {
	t.EndTime = time.Now()
	t.Duration = t.EndTime.Sub(t.StartTime)
	t.ExitCode = exitCode

	// Determine final status based on exit code and output
	if exitCode == 0 {
		// Check for errors/warnings in output
		hasErrors, hasWarnings := t.hasOutputIssues()

		if hasErrors {
			t.Status = StatusError
		} else if hasWarnings {
			t.Status = StatusWarning
		} else {
			t.Status = StatusSuccess
		}
	} else {
		t.Status = StatusError
	}
}

// hasOutputIssues checks if the output contains errors or warnings
func (t *Task) hasOutputIssues() (hasErrors, hasWarnings bool) {
	for _, line := range t.OutputLines {
		if line.Type == TypeError {
			hasErrors = true
		} else if line.Type == TypeWarning {
			hasWarnings = true
		}
	}
	return
}

// UpdateTaskContext updates the task's cognitive context based on output analysis
func (t *Task) UpdateTaskContext() {
	// Count issues to determine cognitive load
	errorCount := 0
	warningCount := 0

	for _, line := range t.OutputLines {
		if line.Type == TypeError {
			errorCount++
		} else if line.Type == TypeWarning {
			warningCount++
		}
	}

	// Set complexity based on output size and issues
	outputSize := len(t.OutputLines)
	if outputSize > 100 {
		t.Context.Complexity = 5
	} else if outputSize > 50 {
		t.Context.Complexity = 4
	} else if outputSize > 20 {
		t.Context.Complexity = 3
	} else {
		t.Context.Complexity = 2
	}

	// Update cognitive load based on research-backed heuristics
	if errorCount > 5 || t.Context.Complexity >= 4 {
		t.Context.CognitiveLoad = LoadHigh
	} else if errorCount > 0 || warningCount > 2 || t.Context.Complexity == 3 {
		t.Context.CognitiveLoad = LoadMedium
	} else {
		t.Context.CognitiveLoad = LoadLow
	}
}

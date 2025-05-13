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
type CognitiveLoadContext string

const (
	LoadLow    CognitiveLoadContext = "low"
	LoadMedium CognitiveLoadContext = "medium"
	LoadHigh   CognitiveLoadContext = "high"
)

// LineType constants for consistent output classification
const (
	TypeDetail   = "detail"
	TypeError    = "error"
	TypeWarning  = "warning"
	TypeSuccess  = "success"
	TypeInfo     = "info"
	TypeProgress = "progress"
	TypeSummary  = "summary"
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
			CognitiveLoad: LoadMedium,
			Complexity:    2,
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

	if exitCode != 0 {
		t.Status = StatusError // Non-zero exit code always means error status
	} else {
		// If exit code is 0, check output lines for issues
		hasErrors, hasWarnings := t.hasOutputIssues()
		if hasErrors {
			t.Status = StatusError // Errors in output override success exit code
		} else if hasWarnings {
			t.Status = StatusWarning
		} else {
			t.Status = StatusSuccess
		}
	}
}

// hasOutputIssues checks if the output contains errors or warnings
func (t *Task) hasOutputIssues() (hasErrors, hasWarnings bool) {
	for _, line := range t.OutputLines {
		switch line.Type {
		case TypeError:
			hasErrors = true
		case TypeWarning:
			hasWarnings = true
			// Note: TypeInfo is not considered an "issue" for status determination
		}
	}
	return
}

// UpdateTaskContext updates the task's cognitive context based on output analysis
func (t *Task) UpdateTaskContext() {
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

	if errorCount > 5 || t.Context.Complexity >= 4 {
		t.Context.CognitiveLoad = LoadHigh
	} else if errorCount > 0 || warningCount > 2 || t.Context.Complexity == 3 {
		t.Context.CognitiveLoad = LoadMedium
	} else {
		t.Context.CognitiveLoad = LoadLow
	}
}

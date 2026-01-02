package dashboard

// Status constants for test/lint results.
const (
	statusRun     = "run"
	statusPass    = "pass"
	statusFail    = "fail"
	statusSkip    = "skip"
	statusError   = "error"
	statusWarning = "warning"
)

// Action constants for go test JSON events.
const (
	actionOutput = "output"
)

// IndicatorStatus represents the status indicator for the task menu.
// Formatters can return this to override exit-code-based status.
type IndicatorStatus int

const (
	// IndicatorDefault uses the task's exit code (success/failure).
	IndicatorDefault IndicatorStatus = iota
	// IndicatorSuccess shows green check regardless of exit code.
	IndicatorSuccess
	// IndicatorWarning shows yellow warning regardless of exit code.
	IndicatorWarning
	// IndicatorError shows red X regardless of exit code.
	IndicatorError
)

// OutputFormatter formats task output for display in the detail panel.
type OutputFormatter interface {
	// Format takes raw output lines and returns formatted content.
	Format(lines []string, width int) string
	// Matches returns true if this formatter should handle the given command.
	Matches(command string) bool
}

// StatusIndicator is an optional interface formatters can implement
// to provide content-aware status indicators for the task menu.
type StatusIndicator interface {
	// GetStatus analyzes output and returns the appropriate indicator status.
	// Return IndicatorDefault to use the task's exit code.
	GetStatus(lines []string) IndicatorStatus
}

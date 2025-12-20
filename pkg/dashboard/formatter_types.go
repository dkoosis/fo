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

// OutputFormatter formats task output for display in the detail panel.
type OutputFormatter interface {
	// Format takes raw output lines and returns formatted content.
	Format(lines []string, width int) string
	// Matches returns true if this formatter should handle the given command.
	Matches(command string) bool
}

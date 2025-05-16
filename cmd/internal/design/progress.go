// cmd/internal/design/progress.go
package design

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// InlineProgress tracks and renders task progress with in-place updates
type InlineProgress struct {
	Task         *Task
	IsActive     bool
	SpinnerIndex int
	StartTime    time.Time
	isTerminal   bool
	mutex        sync.Mutex
	Debug        bool
}

// NewInlineProgress creates a progress tracker for a task
func NewInlineProgress(task *Task, debugMode bool) *InlineProgress {
	// Detect if we're in CI mode from the task config
	isCIMode := task.Config.IsMonochrome && task.Config.Style.NoTimer

	// Determine if we're in a terminal
	isTerminal := IsInteractiveTerminal()

	// Force non-interactive mode for CI regardless of terminal status
	if isCIMode {
		isTerminal = false
	}

	return &InlineProgress{
		Task:         task,
		IsActive:     false,
		SpinnerIndex: 0,
		StartTime:    time.Now(),
		isTerminal:   isTerminal,
		Debug:        debugMode,
	}
}

// IsInteractiveTerminal checks if stdout is connected to a terminal
func IsInteractiveTerminal() bool {
	fd := int(os.Stdout.Fd())
	isTerminal := term.IsTerminal(fd)
	return isTerminal
}

// Start begins progress tracking and renders initial state
func (p *InlineProgress) Start(ctx context.Context, enableSpinner bool) {
	p.mutex.Lock()
	p.IsActive = true
	p.StartTime = time.Now()
	p.mutex.Unlock()

	// Render initial state
	p.RenderProgress("running")

	// Start spinner if enabled and in a terminal (never in CI mode)
	if enableSpinner && p.isTerminal && !p.Task.Config.IsMonochrome {
		go p.runSpinner(ctx)
	}
}

// Complete marks the progress as complete and renders final state
func (p *InlineProgress) Complete(status string) {
	p.mutex.Lock()
	p.IsActive = false
	p.mutex.Unlock()

	// Render final state
	p.RenderProgress(status)
}

// RenderProgress updates the terminal with the current progress state
func (p *InlineProgress) RenderProgress(status string) {
	// Generate formatted message based on task and status
	message := p.formatProgressMessage(status)

	// In terminal mode with color support, update in-place
	if p.isTerminal && !p.Task.Config.IsMonochrome {
		fmt.Print("\r\033[K") // Carriage return + erase line
		fmt.Print(message)

		// Add newline for completed states
		if status != "running" {
			fmt.Println()
		}
	} else {
		// Non-terminal mode or CI mode, just print new lines
		fmt.Println(message)
	}
}

// formatProgressMessage creates the formatted status line
func (p *InlineProgress) formatProgressMessage(status string) string {
	usePlainAscii := p.Task.Config.IsMonochrome || !p.isTerminal

	toolLabel := p.Task.Label
	if toolLabel == "" {
		toolLabel = filepath.Base(p.Task.Command) // Fallback if label is empty
	}
	commandBaseName := filepath.Base(p.Task.Command)
	formattedDuration := formatDuration(p.Task.Duration) // For completed states

	// Running state for spinner
	if status == "running" {
		if usePlainAscii {
			baseVerb := getBaseVerb(p.Task.Intent)
			target := getTargetDescription(p.Task.Command, p.Task.Args)
			actionPhrase := fmt.Sprintf("%s %s", baseVerb, target)
			return fmt.Sprintf("[BUSY] %s: %s [Working...]", toolLabel, actionPhrase)
		}

		// Themed running state (spinner animation)
		indent := ""
		if p.Task.Config.Style.UseBoxes && !p.Task.Config.IsMonochrome {
			indent = p.Task.Config.Border.VerticalChar + " "
		}

		baseVerb := getBaseVerb(p.Task.Intent)
		target := getTargetDescription(p.Task.Command, p.Task.Args)
		actionPhrase := fmt.Sprintf("%s %s", baseVerb, target)

		p.mutex.Lock()
		spinnerChars := "-\\|/"
		if elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line"); elemStyle.AdditionalChars != "" {
			spinnerChars = elemStyle.AdditionalChars
		}
		if len(spinnerChars) == 0 {
			spinnerChars = "-\\|/"
		}
		spinnerIcon := string(spinnerChars[p.SpinnerIndex])
		p.mutex.Unlock()

		runningColor := p.Task.Config.GetColor("Process")
		runningDuration := formatDuration(time.Since(p.StartTime))

		return fmt.Sprintf("%s%s %s: %s%s%s [%sWorking%s %s]",
			indent,
			spinnerIcon,
			toolLabel,
			runningColor,
			actionPhrase,
			p.Task.Config.ResetColor(),
			runningColor,
			p.Task.Config.ResetColor(),
			runningDuration)
	}

	// Completed states (success, error, warning) - UPDATED FORMATTING
	var icon string
	// var statusColorKey string // Not directly used for the bracketed part's color anymore

	switch status {
	case "success":
		icon = p.Task.Config.GetIcon("Success")
		// statusColorKey = "Success" // Retained if other parts of the line need it
	case "error":
		icon = p.Task.Config.GetIcon("Error")
		// statusColorKey = "Error"
	case "warning":
		icon = p.Task.Config.GetIcon("Warning")
		// statusColorKey = "Warning"
	default:
		icon = p.Task.Config.GetIcon("Info")
		// statusColorKey = "Process"
	}

	if usePlainAscii {
		var plainStatusPrefix string
		switch status {
		case "success":
			plainStatusPrefix = "[OK]"
		case "error":
			plainStatusPrefix = "[ERROR]"
		case "warning":
			plainStatusPrefix = "[WARN]"
		default:
			plainStatusPrefix = "[INFO]"
		}
		return fmt.Sprintf("%s %s [%s, %s]",
			plainStatusPrefix,
			toolLabel,
			commandBaseName,
			formattedDuration)
	}

	// Themed completed state - UPDATED for muted bracketed part
	indent := ""
	if p.Task.Config.Style.UseBoxes && !p.Task.Config.IsMonochrome {
		indent = p.Task.Config.Border.VerticalChar + " "
	}

	mutedColor := p.Task.Config.GetColor("Muted") // Get the "Muted" color
	resetColor := p.Task.Config.ResetColor()

	// Themed: INDENT ICON TASK_LABEL MUTED_COLOR[COMMAND_BASENAME, DURATION]RESET_COLOR
	return fmt.Sprintf("%s%s %s %s[%s, %s]%s", // Added space before mutedColor
		indent,
		icon,
		toolLabel,         // e.g., "Lint YAML files"
		mutedColor,        // Apply muted color to the whole bracketed section
		commandBaseName,   // e.g., "yamllint"
		formattedDuration, // e.g., "55ms"
		resetColor)        // Reset color after the bracketed section
}

// runSpinner animates the spinner while the task is running
func (p *InlineProgress) runSpinner(ctx context.Context) {
	// This function should ONLY run if p.isTerminal is true and not monochrome,
	// as typically gated by the Start method.
	// However, the debug prints for state are still useful.

	// Default to ASCII spinner for maximum compatibility
	spinnerChars := "-\\|/" // DEFAULT_ASCII_SPINNER

	// --- Debug: Initial state ---
	if p.Debug { // Check the global debug flag
		fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] runSpinner Init: isTerminal: %t, Config.IsMonochrome: %t, Config.ThemeName: %s\n",
			p.isTerminal, p.Task.Config.IsMonochrome, p.Task.Config.ThemeName)
	}

	// Determine spinner characters
	if !p.Task.Config.IsMonochrome && p.isTerminal {
		elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line")
		if elemStyle.AdditionalChars != "" {
			spinnerChars = elemStyle.AdditionalChars
			if p.Debug { // Check the global debug flag
				fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Source: Theme ('%s') Task_Progress_Line.additional_chars: \"%s\"\n", p.Task.Config.ThemeName, spinnerChars)
			}
		} else {
			if p.Debug { // Check the global debug flag
				fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Source: Default ASCII (Theme '%s' Task_Progress_Line.AdditionalChars was empty).\n", p.Task.Config.ThemeName)
			}
		}
	} else {
		if p.Debug { // Check the global debug flag
			fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Source: Default ASCII (Conditions for Unicode not met. IsMonochrome: %t, isTerminal: %t).\n", p.Task.Config.IsMonochrome, p.isTerminal)
		}
	}

	// Ensure we have at least one character for the spinner animation
	if len(spinnerChars) == 0 {
		spinnerChars = "-\\|/" // Absolute fallback
		if p.Debug {           // Check the global debug flag
			fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Fallback: spinnerChars was empty, reset to absolute default ASCII: \"%s\"\n", spinnerChars)
		}
	}

	if p.Debug { // Check the global debug flag
		fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Final Chars: \"%s\"\n", spinnerChars)
	}

	// Determine spinner interval
	interval := 180 * time.Millisecond // Default interval
	if p.Task.Config.Style.SpinnerInterval > 0 {
		interval = time.Duration(p.Task.Config.Style.SpinnerInterval) * time.Millisecond
	}
	if p.Debug { // Check the global debug flag
		fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Interval: %v\n", interval)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if p.Debug { // Check the global debug flag
				fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Event: Context done, exiting spinner loop.\n")
			}
			return
		case <-ticker.C:
			p.mutex.Lock()
			isActive := p.IsActive
			p.mutex.Unlock()

			if !isActive {
				if p.Debug { // Check the global debug flag
					fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Event: Task no longer active, exiting spinner loop.\n")
				}
				return
			}

			// Update spinner index
			p.mutex.Lock()
			p.SpinnerIndex = (p.SpinnerIndex + 1) % len(spinnerChars)
			// Optional: Debug for each frame, can be very verbose
			// if p.Debug {
			// 	currentSpinnerCharForFrame := string(spinnerChars[p.SpinnerIndex])
			// 	fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Frame: Index: %d, Char: '%s'\n", p.SpinnerIndex, currentSpinnerCharForFrame)
			// }
			p.mutex.Unlock()

			// Re-render the progress line with the new spinner char and updated time
			p.RenderProgress("running")
		}
	}
}

// Helper functions for the new format

// getBaseVerb converts an intent (potentially with -ing suffix) to base form
func getBaseVerb(intent string) string {
	if intent == "" {
		return "run"
	}

	// Convert to lowercase for better matching
	intentLower := strings.ToLower(intent)

	// Common verb mapping for exact matches
	verbMap := map[string]string{
		"building":     "build",
		"running":      "run",
		"testing":      "test",
		"checking":     "check",
		"linting":      "lint",
		"formatting":   "format",
		"tidying":      "tidy",
		"downloading":  "download",
		"verifying":    "verify",
		"vetting":      "vet",
		"installing":   "install",
		"analyzing":    "analyze",
		"generating":   "generate",
		"compiling":    "compile",
		"benchmarking": "benchmark",
		"validating":   "validate",
		"migrating":    "migrate",
		"deploying":    "deploy",
		"publishing":   "publish",
		"cleaning":     "clean",
		"executing":    "execute",
		"searching":    "search",
		"filtering":    "filter",
		"changing":     "change",
		"transforming": "transform",
		"processing":   "process",
		"compressing":  "compress",
		"archiving":    "archive",
		"starting":     "start",
	}

	// Check direct map first
	if baseForm, ok := verbMap[intentLower]; ok {
		return baseForm
	}

	// Handle -ing suffix removal for verbs not in the map
	if strings.HasSuffix(intentLower, "ing") {
		// Special cases where simple removal of -ing doesn't work
		if strings.HasSuffix(intentLower, "ying") {
			return intentLower[:len(intentLower)-4] + "y"
		}
		if strings.HasSuffix(intentLower, "ting") && len(intentLower) > 4 &&
			!strings.HasSuffix(intentLower, "ating") &&
			!strings.HasSuffix(intentLower, "cting") &&
			!strings.HasSuffix(intentLower, "sting") {
			return intentLower[:len(intentLower)-4] + "t"
		}
		if strings.HasSuffix(intentLower, "ping") {
			return intentLower[:len(intentLower)-4] + "p"
		}
		if strings.HasSuffix(intentLower, "ding") {
			return intentLower[:len(intentLower)-4] + "d"
		}
		if strings.HasSuffix(intentLower, "ming") {
			return intentLower[:len(intentLower)-4] + "m"
		}
		if strings.HasSuffix(intentLower, "ning") {
			return intentLower[:len(intentLower)-4] + "n"
		}
		if strings.HasSuffix(intentLower, "king") {
			return intentLower[:len(intentLower)-4] + "k"
		}

		// General case - just remove -ing
		return intentLower[:len(intentLower)-3]
	}

	// If not ending with -ing, return as is
	return intentLower
}

// getTargetDescription returns a suitable target description based on command and args
func getTargetDescription(cmd string, args []string) string {
	// Extract basename of command
	cmdBase := cmd
	if lastSlash := strings.LastIndex(cmd, "/"); lastSlash >= 0 {
		cmdBase = cmd[lastSlash+1:]
	}

	// Known command-target mappings
	switch cmdBase {
	case "go":
		if len(args) > 0 {
			switch args[0] {
			case "build":
				if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
					return args[1]
				}
				return "binary"
			case "test":
				return "tests"
			case "mod":
				if len(args) > 1 {
					return "go.mod"
				}
				return "modules"
			case "vet":
				return "code"
			case "generate":
				return "code"
			case "run":
				if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
					return args[1]
				}
				return "program"
			case "install":
				if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
					return args[1]
				}
				return "package"
			}
			return args[0]
		}
		return "code"

	case "golangci-lint":
		if len(args) > 0 {
			if args[0] == "run" {
				return "code"
			}
			if args[0] == "fmt" {
				return "code"
			}
			return args[0]
		}
		return "code"

	case "yamllint":
		return "YAML files"

	case "make":
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			return args[0]
		}
		return "target"

	case "docker":
		if len(args) > 0 {
			if args[0] == "build" {
				return "image"
			}
			if args[0] == "run" {
				return "container"
			}
			return args[0]
		}
		return "container"
	}

	// Generic approach for other commands
	// Try to find the first non-flag argument
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			// Check if it looks like a filename or path
			if strings.Contains(arg, ".") || strings.Contains(arg, "/") {
				return arg
			}
			// Otherwise, use it as is if it's not empty
			if arg != "" {
				return arg
			}
		}
	}

	// If we can't find a suitable argument, use a generic target
	return "target"
}

// Helper function to capitalize first letter
// nolint: unused
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

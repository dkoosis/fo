// cmd/internal/design/progress.go
package design

import (
	"context"
	"fmt"
	"os"
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
}

// NewInlineProgress creates a progress tracker for a task
func NewInlineProgress(task *Task) *InlineProgress {
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

// formatProgressMessage creates the formatted status line following the new design:
// [ICON] <ToolLabel>: <ActionPhrase> [<OutcomeWord> <Duration>]
// formatProgressMessage creates the formatted status line
func (p *InlineProgress) formatProgressMessage(status string) string {
	// Determine if we should use plain ASCII characters
	// Use ASCII if:
	// 1. Config.IsMonochrome is true (explicitly set by --no-color, --ci, or theme)
	// OR
	// 2. We are not in an interactive terminal (p.isTerminal is false)
	usePlainAscii := p.Task.Config.IsMonochrome || !p.isTerminal

	// Handle plain ASCII mode (monochrome or non-interactive terminal)
	if usePlainAscii {
		toolLabel := p.Task.Label
		if toolLabel == "" {
			toolLabel = p.Task.Command
		}

		// Get verb and target
		baseVerb := getBaseVerb(p.Task.Intent)
		target := getTargetDescription(p.Task.Command, p.Task.Args)
		actionPhrase := fmt.Sprintf("%s %s", baseVerb, target)

		// Format based on status
		switch status {
		case "running":
			// For static line when spinner isn't running
			return fmt.Sprintf("[BUSY] %s: %s [Working...]", toolLabel, actionPhrase)
		case "success":
			return fmt.Sprintf("[OK] %s: %s [OK %s]", toolLabel, actionPhrase, formatDuration(p.Task.Duration))
		case "error":
			return fmt.Sprintf("[ERROR] %s: %s [Failed %s]", toolLabel, actionPhrase, formatDuration(p.Task.Duration))
		case "warning":
			return fmt.Sprintf("[WARN] %s: %s [Warning %s]", toolLabel, actionPhrase, formatDuration(p.Task.Duration))
		default:
			return fmt.Sprintf("[INFO] %s: %s [Done %s]", toolLabel, actionPhrase, formatDuration(p.Task.Duration))
		}
	}

	// Regular formatting for normal terminal mode with color/Unicode
	indent := ""
	if p.Task.Config.Style.UseBoxes {
		indent = p.Task.Config.Border.VerticalChar + " "
	}

	// Get tool label
	toolLabel := p.Task.Label
	if toolLabel == "" {
		toolLabel = p.Task.Command
	}

	// Get verb and target
	baseVerb := getBaseVerb(p.Task.Intent)
	target := getTargetDescription(p.Task.Command, p.Task.Args)
	actionPhrase := fmt.Sprintf("%s %s", baseVerb, target)

	// Variables for icon, color, and outcome text
	var icon, colorCode, outcomeWord, duration string

	// Format duration for completed states
	if status != "running" {
		duration = formatDuration(p.Task.Duration)
	} else {
		duration = formatDuration(time.Since(p.StartTime))
	}

	// For spinner in running state, get the current spinner character
	if status == "running" {
		// Use current spinner position
		p.mutex.Lock()
		spinnerChars := "-\\|/" // Default fallback
		if elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line"); elemStyle.AdditionalChars != "" {
			spinnerChars = elemStyle.AdditionalChars
		}
		if len(spinnerChars) > 0 && p.SpinnerIndex < len(spinnerChars) {
			icon = string(spinnerChars[p.SpinnerIndex])
		} else {
			icon = "-" // Fallback
		}
		p.mutex.Unlock()

		colorCode = p.Task.Config.GetColor("Process")
		outcomeWord = "Working"
	} else {
		// Choose icon, color, and outcome word based on static status
		switch status {
		case "success":
			icon = p.Task.Config.GetIcon("Success")
			colorCode = p.Task.Config.GetColor("Success")
			outcomeWord = "OK"
		case "error":
			icon = p.Task.Config.GetIcon("Error")
			colorCode = p.Task.Config.GetColor("Error")
			outcomeWord = "Error"
		case "warning":
			icon = p.Task.Config.GetIcon("Warning")
			colorCode = p.Task.Config.GetColor("Warning")
			outcomeWord = "Warning"
		default:
			icon = p.Task.Config.GetIcon("Info")
			colorCode = p.Task.Config.GetColor("Process")
			outcomeWord = "Done"
		}
	}

	// Format the output line
	return fmt.Sprintf("%s%s %s: %s%s%s [%s%s%s %s]",
		indent,
		icon,
		toolLabel,
		colorCode,
		actionPhrase,
		p.Task.Config.ResetColor(),
		colorCode,
		outcomeWord,
		p.Task.Config.ResetColor(),
		duration)
}

// runSpinner animates the spinner while the task is running
func (p *InlineProgress) runSpinner(ctx context.Context) {
	// This function should ONLY run if p.isTerminal is true
	// The Start method already gates this with:
	// if enableSpinner && p.isTerminal && !p.Task.Config.IsMonochrome

	// Default to ASCII spinner for maximum compatibility
	spinnerChars := "-\\|/"

	// Only use Unicode spinner if explicitly configured, in a terminal, and not in monochrome mode
	if !p.Task.Config.IsMonochrome && p.isTerminal {
		if elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line"); elemStyle.AdditionalChars != "" {
			spinnerChars = elemStyle.AdditionalChars
		}
	}

	// Ensure we have at least one character
	if len(spinnerChars) == 0 {
		spinnerChars = "-\\|/"
	}

	// Default interval
	interval := 180 * time.Millisecond
	if p.Task.Config.Style.SpinnerInterval > 0 {
		interval = time.Duration(p.Task.Config.Style.SpinnerInterval) * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mutex.Lock()
			isActive := p.IsActive
			p.mutex.Unlock()

			if !isActive {
				return
			}

			// Update spinner index
			p.mutex.Lock()
			p.SpinnerIndex = (p.SpinnerIndex + 1) % len(spinnerChars)
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

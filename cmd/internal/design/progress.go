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
func (p *InlineProgress) formatProgressMessage(status string) string {
	// Special handling for CI mode (text-only output)
	if p.Task.Config.IsMonochrome {
		toolLabel := p.Task.Label
		if toolLabel == "" {
			toolLabel = p.Task.Command
		}

		// Use simple bracketed format for CI mode
		switch status {
		case "running":
			return fmt.Sprintf("[START] %s: working...", toolLabel)
		case "success":
			return fmt.Sprintf("[SUCCESS] %s: done (%s)", toolLabel, formatDuration(p.Task.Duration))
		case "error":
			return fmt.Sprintf("[FAILED] %s: error (%s)", toolLabel, formatDuration(p.Task.Duration))
		case "warning":
			return fmt.Sprintf("[WARNING] %s: warning (%s)", toolLabel, formatDuration(p.Task.Duration))
		default:
			return fmt.Sprintf("[INFO] %s: done (%s)", toolLabel, formatDuration(p.Task.Duration))
		}
	}

	// Regular formatting for normal terminal mode
	indent := ""
	if p.Task.Config.Style.UseBoxes {
		indent = p.Task.Config.Border.VerticalChar + " "
	}

	// Get tool label (use command basename if label is empty)
	toolLabel := p.Task.Label
	if toolLabel == "" {
		toolLabel = p.Task.Command
	}

	// Get verb from intent (base form, not -ing form)
	baseVerb := getBaseVerb(p.Task.Intent)

	// Get target description (usually first non-flag argument or command-specific target)
	target := getTargetDescription(p.Task.Command, p.Task.Args)

	// Format action phrase
	actionPhrase := fmt.Sprintf("%s %s", baseVerb, target)

	// Variables for icon, color, and outcome text
	var icon, colorCode, outcomeWord, duration string

	// Format duration for completed states
	if status != "running" {
		duration = formatDuration(p.Task.Duration)
	} else {
		duration = formatDuration(time.Since(p.StartTime))
	}

	// Choose icon, color, and outcome word based on status
	switch status {
	case "running":
		icon = p.Task.Config.GetIcon("Start")
		colorCode = p.Task.Config.GetColor("Process")
		outcomeWord = "Working"

		return fmt.Sprintf("%s%s %s: %s%s%s [%s %s]",
			indent,
			icon,
			toolLabel,
			colorCode,
			actionPhrase,
			p.Task.Config.ResetColor(),
			outcomeWord,
			duration)

	case "success":
		icon = p.Task.Config.GetIcon("Success")
		colorCode = p.Task.Config.GetColor("Success")
		outcomeWord = "OK"

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

	case "error":
		icon = p.Task.Config.GetIcon("Error")
		colorCode = p.Task.Config.GetColor("Error")
		outcomeWord = "Error"

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

	case "warning":
		icon = p.Task.Config.GetIcon("Warning")
		colorCode = p.Task.Config.GetColor("Warning")
		outcomeWord = "Warning"

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

	default:
		icon = p.Task.Config.GetIcon("Info")
		colorCode = p.Task.Config.GetColor("Process")
		outcomeWord = "Done"

		return fmt.Sprintf("%s%s %s: %s%s%s [%s %s]",
			indent,
			icon,
			toolLabel,
			colorCode,
			actionPhrase,
			p.Task.Config.ResetColor(),
			outcomeWord,
			duration)
	}
}

// runSpinner animates the spinner while the task is running
func (p *InlineProgress) runSpinner(ctx context.Context) {
	// Pulsing heartbeat spinner as preferred
	spinnerChars := "•⦿⦿⦿•⦿⦿⦿"

	// Only use custom spinner if configured in config file and not in monochrome mode
	if !p.Task.Config.IsMonochrome && p.isTerminal {
		// Check if custom spinner is defined in config
		if elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line"); elemStyle.AdditionalChars != "" {
			// Use the configured spinner if it's not empty
			if len(elemStyle.AdditionalChars) > 0 {
				spinnerChars = elemStyle.AdditionalChars
			}
		}
	} else if p.Task.Config.IsMonochrome {
		// Use ASCII spinner for monochrome/CI mode
		spinnerChars = "-\\|/"
	}

	// Ensure we have at least one character
	if len(spinnerChars) == 0 {
		spinnerChars = "•⦿⦿⦿•⦿⦿⦿"
	}

	// Use a moderate interval that allows the spinner to be visible
	// but still feels responsive
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
			currSpinChar := string(spinnerChars[p.SpinnerIndex])
			p.mutex.Unlock()

			// Generate message with current spinner frame and updated time
			indent := ""
			if p.Task.Config.Style.UseBoxes {
				indent = p.Task.Config.Border.VerticalChar + " "
			}

			// Get tool label
			toolLabel := p.Task.Label
			if toolLabel == "" {
				toolLabel = p.Task.Command
			}

			// Get verb from intent (base form, not -ing form)
			baseVerb := getBaseVerb(p.Task.Intent)

			// Get target description
			target := getTargetDescription(p.Task.Command, p.Task.Args)

			// Format action phrase
			actionPhrase := fmt.Sprintf("%s %s", baseVerb, target)

			colorCode := p.Task.Config.GetColor("Process")
			duration := formatDuration(time.Since(p.StartTime))

			message := fmt.Sprintf("%s%s %s: %s%s%s [Working %s]",
				indent,
				currSpinChar,
				toolLabel,
				colorCode,
				actionPhrase,
				p.Task.Config.ResetColor(),
				duration)

			// Update display
			fmt.Print("\r\033[K") // Carriage return + erase line
			fmt.Print(message)
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

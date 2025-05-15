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

// formatProgressMessage creates the formatted status line
func (p *InlineProgress) formatProgressMessage(status string) string {
	// Special handling for CI mode (text-only output)
	if p.Task.Config.IsMonochrome {
		subject := p.Task.Label
		if subject == "" {
			subject = p.Task.Command
		}

		// Use simple bracketed format for CI mode
		switch status {
		case "running":
			return fmt.Sprintf("[START] %s...", subject)
		case "success":
			return fmt.Sprintf("[SUCCESS] %s", subject)
		case "error", "warning":
			return fmt.Sprintf("[FAILED] %s", subject)
		default:
			return fmt.Sprintf("[INFO] %s", subject)
		}
	}

	// Regular formatting for normal terminal mode
	indent := ""
	if p.Task.Config.Style.UseBoxes {
		indent = p.Task.Config.Border.VerticalChar + " "
	}

	var icon, colorCode, verb, subject, duration string

	// Get subject (task name or command)
	subject = p.Task.Label
	if subject == "" {
		subject = p.Task.Command
	}

	// Get verb from intent or default
	verb = p.Task.Intent
	if verb == "" {
		verb = "Process"
	}

	// Format duration for completed states
	if status != "running" {
		duration = formatDuration(p.Task.Duration)
	}

	// Choose icon, color, and template based on status
	switch status {
	case "running":
		icon = p.Task.Config.GetIcon("Start")
		colorCode = p.Task.Config.GetColor("Process")
		return fmt.Sprintf("%s%s %s%sing %s...%s",
			indent,
			icon,
			colorCode,
			capitalizeFirst(verb),
			subject,
			p.Task.Config.ResetColor())

	case "success":
		icon = p.Task.Config.GetIcon("Success")
		colorCode = p.Task.Config.GetColor("Success")
		return fmt.Sprintf("%s%s %s%sing %s complete%s (%s)",
			indent,
			icon,
			colorCode,
			capitalizeFirst(verb),
			subject,
			p.Task.Config.ResetColor(),
			duration)

	case "error", "warning":
		icon = p.Task.Config.GetIcon("Error")
		colorCode = p.Task.Config.GetColor("Error")
		return fmt.Sprintf("%s%s %s%sing %s failed%s (%s)",
			indent,
			icon,
			colorCode,
			capitalizeFirst(verb),
			subject,
			p.Task.Config.ResetColor(),
			duration)

	default:
		icon = p.Task.Config.GetIcon("Info")
		colorCode = p.Task.Config.GetColor("Process")
		return fmt.Sprintf("%s%s %s%s%s (%s)",
			indent,
			icon,
			colorCode,
			subject,
			p.Task.Config.ResetColor(),
			duration)
	}
}

// runSpinner animates the spinner while the task is running
func (p *InlineProgress) runSpinner(ctx context.Context) {
	// Use simple ASCII spinner by default for maximum compatibility
	spinnerChars := "-\\|/"

	// Only use Unicode spinner if configured and in a suitable terminal
	if !p.Task.Config.IsMonochrome && p.isTerminal {
		// Check if custom spinner is defined in config
		if elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line"); elemStyle.AdditionalChars != "" {
			// If AdditionalChars contains ASCII characters, use them
			if strings.ContainsAny(elemStyle.AdditionalChars, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") {
				spinnerChars = "-\\|/" // Fall back to ASCII if invalid Unicode
			} else {
				spinnerChars = elemStyle.AdditionalChars
			}
		}
	}

	// Ensure we have at least one character
	if len(spinnerChars) == 0 {
		spinnerChars = "-\\|/"
	}

	// Default interval of 80ms unless configured otherwise
	interval := 80 * time.Millisecond
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

			// Generate message with current spinner frame
			indent := ""
			if p.Task.Config.Style.UseBoxes {
				indent = p.Task.Config.Border.VerticalChar + " "
			}

			verb := p.Task.Intent
			if verb == "" {
				verb = "Process"
			}

			subject := p.Task.Label
			if subject == "" {
				subject = p.Task.Command
			}

			colorCode := p.Task.Config.GetColor("Process")

			message := fmt.Sprintf("%s%s %s%sing %s...%s",
				indent,
				currSpinChar,
				colorCode,
				capitalizeFirst(verb),
				subject,
				p.Task.Config.ResetColor())

			// Update display
			fmt.Print("\r\033[K") // Carriage return + erase line
			fmt.Print(message)
		}
	}
}

// Helper function to capitalize first letter
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Complete progress.go file with unused functions removed

package design

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
			// Simplified format - just show task label and status
			return fmt.Sprintf("[BUSY] %s [Working...]", toolLabel)
		}

		// Themed running state (spinner animation)
		indent := ""
		if p.Task.Config.Style.UseBoxes && !p.Task.Config.IsMonochrome {
			indent = p.Task.Config.Border.VerticalChar + " "
		}

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

		// Simplified format - no action phrase
		return fmt.Sprintf("%s%s %s [%sWorking%s %s]",
			indent,
			spinnerIcon,
			toolLabel,
			runningColor,
			p.Task.Config.ResetColor(),
			runningDuration)
	}

	// Completed states (success, error, warning)
	var icon string

	switch status {
	case "success":
		icon = p.Task.Config.GetIcon("Success")
	case "error":
		icon = p.Task.Config.GetIcon("Error")
	case "warning":
		icon = p.Task.Config.GetIcon("Warning")
	default:
		icon = p.Task.Config.GetIcon("Info")
	}

	if usePlainAscii {
		var plainStatusPrefix string
		switch status {
		case "success":
			plainStatusPrefix = "[OK]"
		case "error":
			plainStatusPrefix = "[ERROR]"
		case "warning":
			plainStatusPrefix = "[WARNING]"
		default:
			plainStatusPrefix = "[INFO]"
		}
		return fmt.Sprintf("%s %s [%s, %s]",
			plainStatusPrefix,
			toolLabel,
			commandBaseName,
			formattedDuration)
	}

	// Themed completed state
	indent := ""
	if p.Task.Config.Style.UseBoxes && !p.Task.Config.IsMonochrome {
		indent = p.Task.Config.Border.VerticalChar + " "
	}

	mutedColor := p.Task.Config.GetColor("Muted")
	resetColor := p.Task.Config.ResetColor()

	return fmt.Sprintf("%s%s %s %s[%s, %s]%s",
		indent,
		icon,
		toolLabel,
		mutedColor,
		commandBaseName,
		formattedDuration,
		resetColor)
}

// runSpinner animates the spinner while the task is running
func (p *InlineProgress) runSpinner(ctx context.Context) {
	// Default to ASCII spinner for maximum compatibility
	spinnerChars := "-\\|/"

	// Debug: Initial state
	if p.Debug {
		fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] runSpinner Init: isTerminal: %t, Config.IsMonochrome: %t, Config.ThemeName: %s\n",
			p.isTerminal, p.Task.Config.IsMonochrome, p.Task.Config.ThemeName)
	}

	// Determine spinner characters
	if !p.Task.Config.IsMonochrome && p.isTerminal {
		elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line")
		if elemStyle.AdditionalChars != "" {
			spinnerChars = elemStyle.AdditionalChars
			if p.Debug {
				fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Source: Theme ('%s') Task_Progress_Line.additional_chars: \"%s\"\n", p.Task.Config.ThemeName, spinnerChars)
			}
		} else {
			if p.Debug {
				fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Source: Default ASCII (Theme '%s' Task_Progress_Line.AdditionalChars was empty).\n", p.Task.Config.ThemeName)
			}
		}
	} else {
		if p.Debug {
			fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Source: Default ASCII (Conditions for Unicode not met. IsMonochrome: %t, isTerminal: %t).\n", p.Task.Config.IsMonochrome, p.isTerminal)
		}
	}

	// Ensure we have at least one character for the spinner animation
	if len(spinnerChars) == 0 {
		spinnerChars = "-\\|/"
		if p.Debug {
			fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Fallback: spinnerChars was empty, reset to absolute default ASCII: \"%s\"\n", spinnerChars)
		}
	}

	if p.Debug {
		fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Final Chars: \"%s\"\n", spinnerChars)
	}

	// Determine spinner interval
	interval := 180 * time.Millisecond
	if p.Task.Config.Style.SpinnerInterval > 0 {
		interval = time.Duration(p.Task.Config.Style.SpinnerInterval) * time.Millisecond
	}
	if p.Debug {
		fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Interval: %v\n", interval)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if p.Debug {
				fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Event: Context done, exiting spinner loop.\n")
			}
			return
		case <-ticker.C:
			p.mutex.Lock()
			isActive := p.IsActive
			p.mutex.Unlock()

			if !isActive {
				if p.Debug {
					fmt.Fprintf(os.Stderr, "[FO_DEBUG_SPINNER] Event: Task no longer active, exiting spinner loop.\n")
				}
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

// Complete progress.go file with unused functions removed

package design

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/term"
)

// InlineProgress tracks and renders task progress with in-place updates.
type InlineProgress struct {
	Task         *Task
	Writer       io.Writer // Output writer for progress messages (defaults to os.Stdout)
	IsActive     bool
	SpinnerIndex int
	StartTime    time.Time
	isTerminal   bool
	mutex        sync.Mutex
	Debug        bool
}

// NewInlineProgress creates a progress tracker for a task.
// The writer parameter specifies where progress output should be written;
// if nil, defaults to os.Stdout. This allows embedding applications to
// capture or redirect progress output.
func NewInlineProgress(task *Task, debugMode bool, writer io.Writer) *InlineProgress {
	// Determine if we're in CI mode:
	// - Prefer explicit CI flag from config
	// - Fall back to heuristic (monochrome + no-timer) for backwards compatibility
	isCIMode := task.Config.CI || (task.Config.IsMonochrome && task.Config.Style.NoTimer)

	// Determine if we're in a terminal
	isTerminal := IsInteractiveTerminal()

	// Force non-interactive mode for CI regardless of terminal status
	if isCIMode {
		isTerminal = false
	}

	// Default to os.Stdout if no writer provided
	if writer == nil {
		writer = os.Stdout
	}

	return &InlineProgress{
		Task:         task,
		Writer:       writer,
		IsActive:     false,
		SpinnerIndex: 0,
		StartTime:    time.Now(),
		isTerminal:   isTerminal,
		Debug:        debugMode,
	}
}

// IsInteractiveTerminal checks if stdout is connected to a terminal.
func IsInteractiveTerminal() bool {
	fd := int(os.Stdout.Fd())
	isTerminal := term.IsTerminal(fd)
	return isTerminal
}

// Start begins progress tracking and renders initial state.
// Note: Cursor hiding is now handled at the console level for better
// restoration guarantees (see fo/console.go runContext).
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

// Complete marks the progress as complete and renders final state.
// Note: Cursor restoration is now handled at the console level via defer
// for better crash recovery (see fo/console.go runContext).
func (p *InlineProgress) Complete(status string) {
	p.mutex.Lock()
	p.IsActive = false
	p.mutex.Unlock()

	// Render final state
	p.RenderProgress(status)
}

// RenderProgress updates the terminal with the current progress state.
func (p *InlineProgress) RenderProgress(status string) {
	// Generate formatted message based on task and status
	message := p.formatProgressMessage(status)

	// In terminal mode with color support, update in-place
	if p.isTerminal && !p.Task.Config.IsMonochrome {
		_, _ = p.Writer.Write([]byte("\r\033[K")) // Carriage return + erase line
		_, _ = p.Writer.Write([]byte(message))

		// Add newline for completed states
		if status != StatusRunning {
			_, _ = p.Writer.Write([]byte("\n"))
		}
	} else {
		// Non-terminal mode or CI mode, just print new lines
		_, _ = p.Writer.Write([]byte(message + "\n"))
	}
}

// formatProgressMessage creates the formatted status line.
func (p *InlineProgress) formatProgressMessage(status string) string {
	usePlainASCII := p.Task.Config.IsMonochrome || !p.isTerminal

	toolLabel := p.Task.Label
	if toolLabel == "" {
		toolLabel = filepath.Base(p.Task.Command) // Fallback if label is empty
	}
	commandBaseName := filepath.Base(p.Task.Command)
	formattedDuration := formatDuration(p.Task.Duration) // For completed states

	// Running state for spinner
	if status == StatusRunning {
		if usePlainASCII {
			// Simplified format - just show task label and status
			return fmt.Sprintf("[BUSY] %s [Working...]", toolLabel)
		}

		// Themed running state (spinner animation)
		indent := ""
		if p.Task.Config.Style.UseBoxes && !p.Task.Config.IsMonochrome {
			indent = p.Task.Config.Border.VerticalChar + " "
		}

		p.mutex.Lock()
		spinnerChars := DefaultSpinnerChars
		if elemStyle := p.Task.Config.GetElementStyle("Task_Progress_Line"); elemStyle.AdditionalChars != "" {
			spinnerChars = elemStyle.AdditionalChars
		}
		if spinnerChars == "" {
			spinnerChars = DefaultSpinnerChars
		}
		// Convert to runes for proper Unicode character indexing
		spinnerRunes := []rune(spinnerChars)
		if len(spinnerRunes) == 0 {
			spinnerRunes = []rune(DefaultSpinnerChars)
		}
		spinnerIcon := string(spinnerRunes[p.SpinnerIndex%len(spinnerRunes)])
		p.mutex.Unlock()

		// Apply pale blue color to spinner icon
		spinnerColor := p.Task.Config.GetColor("PaleBlue")
		if spinnerColor == "" {
			// Fallback to Process color if PaleBlue not available
			spinnerColor = p.Task.Config.GetColor("Process")
		}
		coloredSpinner := string(spinnerColor) + spinnerIcon + string(p.Task.Config.ResetColor())

		runningColor := p.Task.Config.GetColor("Process")
		runningDuration := formatDuration(time.Since(p.StartTime))

		// Simplified format - no action phrase
		return fmt.Sprintf("%s%s %s [%sWorking%s %s]",
			indent,
			coloredSpinner,
			toolLabel,
			runningColor,
			p.Task.Config.ResetColor(),
			runningDuration)
	}

	// Completed states (success, error, warning)
	var icon string

	switch status {
	case StatusSuccess:
		icon = p.Task.Config.GetIcon("Success")
	case StatusError:
		icon = p.Task.Config.GetIcon("Error")
	case StatusWarning:
		icon = p.Task.Config.GetIcon("Warning")
	default:
		icon = p.Task.Config.GetIcon("Info")
	}

	if usePlainASCII {
		var plainStatusPrefix string
		switch status {
		case StatusSuccess:
			plainStatusPrefix = "[OK]"
		case StatusError:
			plainStatusPrefix = "[ERROR]"
		case StatusWarning:
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

// runSpinner animates the spinner while the task is running.
func (p *InlineProgress) runSpinner(ctx context.Context) {
	// Default to ASCII spinner for maximum compatibility
	spinnerChars := DefaultSpinnerChars

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
				fmt.Fprintf(os.Stderr,
					"[FO_DEBUG_SPINNER] Source: Theme ('%s') Task_Progress_Line.additional_chars: \"%s\"\n",
					p.Task.Config.ThemeName, spinnerChars)
			}
		} else if p.Debug {
			fmt.Fprintf(os.Stderr,
				"[FO_DEBUG_SPINNER] Source: Default ASCII (Theme '%s' AdditionalChars was empty).\n",
				p.Task.Config.ThemeName)
		}
	} else if p.Debug {
		fmt.Fprintf(os.Stderr,
			"[FO_DEBUG_SPINNER] Source: Default ASCII (IsMonochrome: %t, isTerminal: %t).\n",
			p.Task.Config.IsMonochrome, p.isTerminal)
	}

	// Ensure we have at least one character for the spinner animation
	if spinnerChars == "" {
		spinnerChars = DefaultSpinnerChars
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

			// Update spinner index (convert to runes for proper Unicode handling)
			p.mutex.Lock()
			spinnerRunes := []rune(spinnerChars)
			if len(spinnerRunes) == 0 {
				spinnerRunes = []rune(DefaultSpinnerChars)
			}
			p.SpinnerIndex = (p.SpinnerIndex + 1) % len(spinnerRunes)
			p.mutex.Unlock()

			// Re-render the progress line with the new spinner char and updated time
			p.RenderProgress("running")
		}
	}
}

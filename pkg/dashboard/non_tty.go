package dashboard

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// RunNonTTY executes tasks and streams prefixed output for non-interactive environments.
func RunNonTTY(ctx context.Context, specs []TaskSpec, out io.Writer) int {
	tasks, updates := StartTasks(ctx, specs)

	// Pre-compute which tasks prefer batch mode (suppress streaming for these)
	prefersBatch := make([]bool, len(tasks))
	for i, task := range tasks {
		prefersBatch[i] = FormatterPrefersBatch(task.Spec.Command)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for update := range updates {
			task := tasks[update.Index]
			if update.Line != "" {
				// Append to task output for later formatting
				task.mu.Lock()
				task.Output = append(task.Output, update.Line)
				task.mu.Unlock()

				// Only stream raw output if formatter doesn't prefer batch mode
				if !prefersBatch[update.Index] {
					prefix := fmt.Sprintf("[%s/%s] ", task.Spec.Group, task.Spec.Name)
					fmt.Fprintf(out, "%s%s\n", prefix, update.Line)
				}
			}
			if update.Status == TaskSuccess || update.Status == TaskFailed {
				task.ExitCode = update.ExitCode
				task.Status = update.Status
			}
		}
	}()
	wg.Wait()
	return renderSummary(out, tasks)
}

func renderSummary(out io.Writer, tasks []*Task) int {
	failures := 0

	// Format task outputs using formatters
	for _, task := range tasks {
		if len(task.Output) > 0 {
			// Use formatter if available
			formatted := FormatOutput(task.Spec.Command, task.Output, 100)
			if formatted != strings.Join(task.Output, "\n") {
				// Formatter produced different output - use it
				fmt.Fprintln(out)
				fmt.Fprintln(out, formatted)
			}
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Summary:")
	for _, task := range tasks {
		// Use content-aware status indicator if available
		indicator := GetIndicatorStatus(task.Spec.Command, task.Output)
		var status string
		var isFailed bool
		switch indicator {
		case IndicatorWarning:
			status = "⚠"
		case IndicatorError:
			status = "✗"
			isFailed = true
		case IndicatorSuccess:
			status = "✓"
		default:
			// Fall back to exit-code-based status
			if task.Status == TaskFailed {
				status = "✗"
				isFailed = true
			} else {
				status = "✓"
			}
		}
		if isFailed {
			failures++
		}
		duration := task.Duration().Round(10 * time.Millisecond)
		fmt.Fprintf(out, "  %s %s/%s (%s)\n", status, task.Spec.Group, task.Spec.Name, duration)
	}
	if failures > 0 {
		fmt.Fprintf(out, "\n%d task(s) failed\n", failures)
		return 1
	}
	return 0
}

// JoinOutput joins buffered output for tests.
func JoinOutput(task *Task) string {
	task.mu.Lock()
	defer task.mu.Unlock()
	return strings.Join(task.Output, "\n")
}

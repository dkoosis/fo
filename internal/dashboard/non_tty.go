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
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for update := range updates {
			task := tasks[update.Index]
			if update.Line != "" {
				prefix := fmt.Sprintf("[%s/%s] ", task.Spec.Group, task.Spec.Name)
				fmt.Fprintf(out, "%s%s\n", prefix, update.Line)
			}
			if update.Status == TaskSuccess || update.Status == TaskFailed {
				task.ExitCode = update.ExitCode
			}
		}
	}()
	wg.Wait()
	return renderSummary(out, tasks)
}

func renderSummary(out io.Writer, tasks []*Task) int {
	failures := 0
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Summary:")
	for _, task := range tasks {
		status := "✓"
		if task.Status == TaskFailed {
			status = "✗"
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

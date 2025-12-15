package dashboard

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"time"
)

// TaskStatus represents runtime state.
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskSuccess
	TaskFailed
)

const defaultBufferLines = 50000

// Task represents execution state.
type Task struct {
	Spec       TaskSpec
	Status     TaskStatus
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
	Output     []string
	mu         sync.Mutex
}

// TaskUpdate describes runtime changes for TUI/non-tty.
type TaskUpdate struct {
	Index      int
	Status     TaskStatus
	Line       string
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
}

// StartTasks starts all tasks concurrently and streams updates.
func StartTasks(ctx context.Context, specs []TaskSpec) ([]*Task, <-chan TaskUpdate) {
	updates := make(chan TaskUpdate)
	tasks := make([]*Task, len(specs))
	var wg sync.WaitGroup
	wg.Add(len(specs))
	for i, spec := range specs {
		tasks[i] = &Task{Spec: spec, Status: TaskPending, ExitCode: -1}
		go runTask(ctx, i, tasks[i], updates, &wg)
	}

	go func() {
		wg.Wait()
		close(updates)
	}()

	return tasks, updates
}

func runTask(ctx context.Context, index int, task *Task, updates chan<- TaskUpdate, wg *sync.WaitGroup) {
	defer wg.Done()
	cmd := exec.CommandContext(ctx, "bash", "-lc", task.Spec.Command)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	task.StartedAt = time.Now()
	task.Status = TaskRunning
	updates <- TaskUpdate{Index: index, Status: TaskRunning, StartedAt: task.StartedAt}

	merged := make(chan string)
	var streamsWG sync.WaitGroup
	streamsWG.Add(2)
	go readStream(&streamsWG, stdout, merged)
	go readStream(&streamsWG, stderr, merged)

	_ = cmd.Start()

	go func() {
		streamsWG.Wait()
		close(merged)
	}()

	for line := range merged {
		// Don't call appendLine here - the TUI/non-tty handler will do it
		// when processing the update to avoid duplicate lines
		updates <- TaskUpdate{Index: index, Status: TaskRunning, Line: line}
	}

	err := cmd.Wait()
	task.FinishedAt = time.Now()
	if err != nil {
		task.Status = TaskFailed
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			task.ExitCode = exitErr.ExitCode()
		} else {
			task.ExitCode = 1
		}
	} else {
		task.Status = TaskSuccess
		task.ExitCode = 0
	}
	updates <- TaskUpdate{Index: index, Status: task.Status, ExitCode: task.ExitCode, FinishedAt: task.FinishedAt}
}

func readStream(wg *sync.WaitGroup, r io.Reader, merged chan<- string) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		merged <- scanner.Text()
	}
}

func (t *Task) appendLine(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Output = append(t.Output, line)
	if len(t.Output) > defaultBufferLines {
		t.Output = t.Output[len(t.Output)-defaultBufferLines:]
	}
}

// GetOutput returns a copy of the output lines (thread-safe).
func (t *Task) GetOutput() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Return a copy to avoid data races
	result := make([]string, len(t.Output))
	copy(result, t.Output)
	return result
}

// Duration returns elapsed time.
func (t *Task) Duration() time.Duration {
	if t.StartedAt.IsZero() {
		return 0
	}
	if t.FinishedAt.IsZero() {
		return time.Since(t.StartedAt)
	}
	return t.FinishedAt.Sub(t.StartedAt)
}

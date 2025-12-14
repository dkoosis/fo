package dashboard

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

func newDefaultRunner(cfg *config) *defaultRunner {
	return &defaultRunner{cfg: cfg}
}

type defaultRunner struct {
	cfg      *config
	writerMu sync.Mutex
}

func (r *defaultRunner) run(ctx context.Context, title string, tasks []TaskSpec) (SuiteResult, error) {
	if len(tasks) == 0 {
		now := time.Now()
		return SuiteResult{StartedAt: now, FinishedAt: now, Tasks: map[string]TaskResult{}}, nil
	}

	isTTY := r.detectTTY()
	r.cfg.ensureRunner()

	suite := SuiteResult{StartedAt: time.Now(), Tasks: make(map[string]TaskResult)}
	var suiteMu sync.Mutex
	var wg sync.WaitGroup
	var failed []string
	var failMu sync.Mutex

	for _, task := range tasks {
		t := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := r.runTask(ctx, t, isTTY)

			suiteMu.Lock()
			suite.Tasks[t.ID] = res
			suiteMu.Unlock()

			if res.Status == Failed && !t.AllowFailure {
				failMu.Lock()
				failed = append(failed, t.ID)
				failMu.Unlock()
			}
		}()
	}

	wg.Wait()
	suite.FinishedAt = time.Now()

	if !isTTY {
		r.printSummary(title, tasks, suite)
	}

	if len(failed) > 0 {
		return suite, aggregatedError{failed: failed}
	}
	return suite, nil
}

func (r *defaultRunner) runTask(ctx context.Context, spec TaskSpec, isTTY bool) TaskResult {
	start := time.Now()
	res := TaskResult{ID: spec.ID, Group: spec.Group, Name: spec.Name, Status: Pending}
	r.emit(Event{Type: EventTaskStarted, TaskID: spec.ID, When: start})

	if len(spec.Command) == 0 || spec.Command[0] == "" {
		res.Status = Failed
		res.Err = errors.New("task has no command")
		res.Duration = time.Since(start)
		r.emit(Event{Type: EventTaskCompleted, TaskID: spec.ID, When: time.Now()})
		return res
	}

	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	cmd.Env = mergeEnv(os.Environ(), spec.Env)

	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter

	scanner := bufio.NewScanner(pipeReader)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	tail := newTailBuffer(r.cfg.maxTail)

	res.Status = Running
	if err := cmd.Start(); err != nil {
		res.Status = Failed
		res.Err = err
		res.Duration = time.Since(start)
		_ = pipeWriter.Close()
		_ = pipeReader.Close()
		r.emit(Event{Type: EventTaskCompleted, TaskID: spec.ID, When: time.Now()})
		return res
	}

	var readWG sync.WaitGroup
	readWG.Add(1)
	go func() {
		defer readWG.Done()
		for scanner.Scan() {
			line := scanner.Text()
			tail.add(line)
			r.emit(Event{Type: EventTaskOutput, TaskID: spec.ID, Line: line, When: time.Now()})
			if !isTTY {
				r.writeStream(spec, line)
			}
		}
	}()

	waitErr := cmd.Wait()
	_ = pipeWriter.Close()
	_ = pipeReader.Close()
	readWG.Wait()

	res.OutputTail = tail.lines()
	res.Duration = time.Since(start)

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			res.ExitCode = exitCode(exitErr)
		}
		res.Status = Failed
		res.Err = waitErr
	} else {
		res.Status = Success
		res.ExitCode = 0
	}

	r.emit(Event{Type: EventTaskCompleted, TaskID: spec.ID, When: time.Now()})
	return res
}

func (r *defaultRunner) emit(evt Event) {
	if r.cfg.onEvent != nil {
		r.cfg.onEvent(evt)
	}
}

func (r *defaultRunner) writeStream(spec TaskSpec, line string) {
	prefix := spec.ID
	if prefix == "" {
		prefix = deriveTaskID(spec.Group, spec.Name)
	}
	r.writerMu.Lock()
	fmt.Fprintf(r.cfg.stdout, "%s | %s\n", prefix, line)
	r.writerMu.Unlock()
}

func (r *defaultRunner) detectTTY() bool {
	if r.cfg.forceTTY != nil {
		return *r.cfg.forceTTY
	}
	type fder interface{ Fd() uintptr }
	if fdw, ok := r.cfg.stdout.(fder); ok {
		return term.IsTerminal(int(fdw.Fd()))
	}
	return false
}

func (r *defaultRunner) printSummary(title string, tasks []TaskSpec, suite SuiteResult) {
	r.writerMu.Lock()
	defer r.writerMu.Unlock()

	if title != "" {
		fmt.Fprintf(r.cfg.stdout, "== %s ==\n", title)
	}

	for _, t := range tasks {
		res := suite.Tasks[t.ID]
		status := "SUCCESS"
		if res.Status != Success {
			status = "FAILED"
			if t.AllowFailure {
				status = "ALLOWED"
			}
		}
		fmt.Fprintf(r.cfg.stdout, "%s | %s | exit=%d | %s\n", res.ID, status, res.ExitCode, res.Duration.String())
	}
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	env := make([]string, len(base))
	copy(env, base)
	for k, v := range extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func exitCode(exitErr *exec.ExitError) int {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}

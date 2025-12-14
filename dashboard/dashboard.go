package dashboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus int

const (
	// Pending indicates the task has not started yet.
	Pending TaskStatus = iota
	// Running indicates the task is currently executing.
	Running
	// Success indicates the task finished with a zero exit code.
	Success
	// Failed indicates the task failed to start or returned non-zero.
	Failed
)

// EventType distinguishes emitted dashboard events.
type EventType int

const (
	EventTaskStarted EventType = iota
	EventTaskOutput
	EventTaskCompleted
)

// Event captures task lifecycle milestones.
type Event struct {
	Type   EventType
	TaskID string
	Line   string
	When   time.Time
}

// TaskSpec defines the configuration for a task to run.
type TaskSpec struct {
	ID           string
	Group        string
	Name         string
	Command      []string
	Env          map[string]string
	Dir          string
	AllowFailure bool
}

// TaskResult is the execution outcome of a task.
type TaskResult struct {
	ID         string
	Group      string
	Name       string
	Status     TaskStatus
	ExitCode   int
	Duration   time.Duration
	OutputTail []string
	Err        error
}

// SuiteResult aggregates all task results for a run.
type SuiteResult struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Tasks      map[string]TaskResult
}

// Option configures a Dashboard.
type Option func(*config)

// WithStdout overrides the dashboard stdout writer.
func WithStdout(w io.Writer) Option {
	return func(cfg *config) { cfg.stdout = w }
}

// WithStderr overrides the dashboard stderr writer.
func WithStderr(w io.Writer) Option {
	return func(cfg *config) { cfg.stderr = w }
}

// WithTTY forces or disables TTY behavior. nil means auto-detect.
func WithTTY(force *bool) Option {
	return func(cfg *config) { cfg.forceTTY = force }
}

// WithMaxTailLines sets the maximum number of tail lines to keep per task.
func WithMaxTailLines(n int) Option {
	return func(cfg *config) { cfg.maxTail = n }
}

// WithOnEvent registers a callback for emitted events.
func WithOnEvent(fn func(Event)) Option {
	return func(cfg *config) { cfg.onEvent = fn }
}

// Dashboard orchestrates multiple tasks.
type Dashboard struct {
	title string
	cfg   config

	mu     sync.Mutex
	groups map[string]struct{}
	tasks  []TaskSpec
}

// New constructs a dashboard instance.
func New(title string, opts ...Option) *Dashboard {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.ensureRunner()
	return &Dashboard{
		title:  title,
		cfg:    cfg,
		groups: make(map[string]struct{}),
	}
}

// AddGroup ensures a group exists for subsequent tasks.
func (d *Dashboard) AddGroup(name string) *Dashboard {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups[name] = struct{}{}
	return d
}

// AddTask adds a task with the provided group, name, and argv.
func (d *Dashboard) AddTask(group string, name string, argv ...string) *Dashboard {
	spec := TaskSpec{Group: group, Name: name, Command: append([]string{}, argv...)}
	return d.AddTaskSpec(spec)
}

// AddTaskSpec adds a fully-specified task.
func (d *Dashboard) AddTaskSpec(spec TaskSpec) *Dashboard {
	d.mu.Lock()
	defer d.mu.Unlock()
	if spec.ID == "" {
		spec.ID = deriveTaskID(spec.Group, spec.Name)
	}
	d.tasks = append(d.tasks, spec)
	return d
}

// Run executes all tasks concurrently and waits for completion.
func (d *Dashboard) Run(ctx context.Context) (SuiteResult, error) {
	return d.cfg.runner.run(ctx, d.title, d.tasks)
}

// deriveTaskID creates a stable identifier for a task.
func deriveTaskID(group, name string) string {
	switch {
	case group != "" && name != "":
		return fmt.Sprintf("%s/%s", group, name)
	case group != "":
		return group
	default:
		return name
	}
}

// aggregatedError collapses multiple failures into one error value.
type aggregatedError struct {
	failed []string
}

func (a aggregatedError) Error() string {
	if len(a.failed) == 0 {
		return ""
	}
	return fmt.Sprintf("%d task(s) failed: %s", len(a.failed), strings.Join(a.failed, ", "))
}

func (a aggregatedError) Failed() []string { return append([]string(nil), a.failed...) }

func defaultConfig() config {
	return config{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		maxTail: 5000,
	}
}

type config struct {
	stdout   io.Writer
	stderr   io.Writer
	forceTTY *bool
	maxTail  int
	onEvent  func(Event)
	runner   taskRunner
}

// ensureRunner sets a default runner if missing.
func (c *config) ensureRunner() {
	if c.runner == nil {
		c.runner = newDefaultRunner(c)
	}
}

// taskRunner abstracts execution for testing.
type taskRunner interface {
	run(ctx context.Context, title string, tasks []TaskSpec) (SuiteResult, error)
}

// ensure imports
var _ taskRunner = (*defaultRunner)(nil)
var _ = errors.Is

package sarif

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dkoosis/fo/pkg/design"
)

// spinnerFrames defines available spinner styles (duplicated from fo to avoid import cycle)
var spinnerFrames = map[string][]string{
	"dots": {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
}

// parseSpinnerChars parses a custom spinner chars string into frames.
func parseSpinnerChars(chars string) []string {
	chars = strings.TrimSpace(chars)
	if chars == "" {
		return nil
	}
	if strings.Contains(chars, " ") {
		return strings.Fields(chars)
	}
	runes := []rune(chars)
	frames := make([]string, 0, len(runes))
	for _, r := range runes {
		frames = append(frames, string(r))
	}
	return frames
}

// ToolSpec defines a tool to run that emits SARIF output.
type ToolSpec struct {
	Name       string   // Display name (e.g., "golangci-lint")
	Command    string   // Command to run (e.g., "golangci-lint")
	Args       []string // Command arguments
	SARIFPath  string   // Path where SARIF output will be written
	WorkingDir string   // Working directory (empty = current)
}

// ToolResult holds the result of running a tool.
type ToolResult struct {
	Spec     ToolSpec
	Document *Document // Parsed SARIF (nil if failed to parse)
	Duration time.Duration
	Error    error // Non-nil if tool failed to run or parse
	ExitCode int
}

// Orchestrator runs multiple tools in parallel with visual feedback.
type Orchestrator struct {
	config    RendererConfig
	foConfig  *design.Config
	writer    io.Writer
	buildDir  string
	spinnerCh string // Custom spinner chars
}

// NewOrchestrator creates an orchestrator with the given configuration.
func NewOrchestrator(config RendererConfig, foConfig *design.Config) *Orchestrator {
	return &Orchestrator{
		config:   config,
		foConfig: foConfig,
		writer:   os.Stdout,
		buildDir: "build",
	}
}

// SetWriter sets the output writer.
func (o *Orchestrator) SetWriter(w io.Writer) {
	o.writer = w
}

// SetBuildDir sets the directory for SARIF output files.
func (o *Orchestrator) SetBuildDir(dir string) {
	o.buildDir = dir
}

// SetSpinnerChars sets custom spinner characters.
func (o *Orchestrator) SetSpinnerChars(chars string) {
	o.spinnerCh = chars
}

// Run executes all tools in parallel and renders results.
func (o *Orchestrator) Run(ctx context.Context, tools []ToolSpec) ([]ToolResult, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	// Ensure build directory exists
	if err := os.MkdirAll(o.buildDir, 0755); err != nil {
		return nil, fmt.Errorf("create build dir: %w", err)
	}

	// Fill in default SARIF paths
	for i := range tools {
		if tools[i].SARIFPath == "" {
			safeName := strings.ReplaceAll(tools[i].Name, "/", "-")
			tools[i].SARIFPath = filepath.Join(o.buildDir, safeName+".sarif")
		}
	}

	// Channel for results
	results := make([]ToolResult, len(tools))
	resultCh := make(chan int, len(tools)) // Index of completed tool

	// Start multi-spinner display
	display := o.newMultiSpinner(tools)
	display.Start()

	// Run all tools in parallel
	var wg sync.WaitGroup
	for i, spec := range tools {
		wg.Add(1)
		go func(idx int, s ToolSpec) {
			defer wg.Done()
			results[idx] = o.runTool(ctx, s)
			resultCh <- idx
		}(i, spec)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Update display as tools complete
	completed := 0
	for idx := range resultCh {
		completed++
		r := results[idx]
		if r.Error != nil {
			display.SetStatus(idx, "error", r.Duration)
		} else if r.Document != nil {
			stats := ComputeStats(r.Document)
			if stats.ByLevel["error"] > 0 {
				display.SetStatus(idx, "issues", r.Duration)
			} else {
				display.SetStatus(idx, "ok", r.Duration)
			}
		} else {
			display.SetStatus(idx, "ok", r.Duration)
		}
	}

	display.Stop()

	// Render results
	o.renderResults(results)

	return results, nil
}

// runTool executes a single tool and returns its result.
func (o *Orchestrator) runTool(ctx context.Context, spec ToolSpec) ToolResult {
	start := time.Now()
	result := ToolResult{Spec: spec}

	// Build command
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if spec.WorkingDir != "" {
		cmd.Dir = spec.WorkingDir
	}

	// Run command
	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			// Non-zero exit is expected for linters with findings
			// Only treat as error if SARIF wasn't produced
		} else {
			result.Error = fmt.Errorf("run %s: %w", spec.Name, err)
			return result
		}
	}

	// Parse SARIF output
	doc, parseErr := ReadFile(spec.SARIFPath)
	if parseErr != nil {
		// If no SARIF but command succeeded, that's fine
		if result.ExitCode == 0 {
			return result
		}
		// If SARIF missing and command failed, report the failure
		result.Error = fmt.Errorf("%s failed (exit %d): %s", spec.Name, result.ExitCode, truncateOutput(output, 200))
		return result
	}

	result.Document = doc
	return result
}

// renderResults outputs the rendered SARIF patterns for all results.
func (o *Orchestrator) renderResults(results []ToolResult) {
	renderer := NewRenderer(o.config, o.foConfig)

	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(o.writer, "\n%s: %v\n", r.Spec.Name, r.Error)
			continue
		}

		if r.Document == nil {
			continue
		}

		stats := ComputeStats(r.Document)
		if stats.TotalIssues == 0 {
			// No issues - just show success message
			icon := o.foConfig.GetIcon("Success")
			fmt.Fprintf(o.writer, "\n%s %s: no issues\n", icon, r.Spec.Name)
			continue
		}

		// Render SARIF patterns
		fmt.Fprintf(o.writer, "\n")
		output := renderer.Render(r.Document)
		fmt.Fprint(o.writer, output)
	}
}

// truncateOutput truncates output to maxLen characters.
func truncateOutput(output []byte, maxLen int) string {
	s := string(output)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// multiSpinner manages multiple spinner lines.
type multiSpinner struct {
	tools     []ToolSpec
	statuses  []string
	durations []time.Duration
	mu        sync.Mutex
	writer    io.Writer
	running   bool
	stopCh    chan struct{}
	doneCh    chan struct{}
	frames    []string
	interval  time.Duration
	frameIdx  int
}

func (o *Orchestrator) newMultiSpinner(tools []ToolSpec) *multiSpinner {
	frames := spinnerFrames["dots"]
	if o.spinnerCh != "" {
		if parsed := parseSpinnerChars(o.spinnerCh); len(parsed) > 0 {
			frames = parsed
		}
	}

	ms := &multiSpinner{
		tools:     tools,
		statuses:  make([]string, len(tools)),
		durations: make([]time.Duration, len(tools)),
		writer:    o.writer,
		frames:    frames,
		interval:  80 * time.Millisecond,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	for i := range tools {
		ms.statuses[i] = "running"
	}

	return ms
}

func (ms *multiSpinner) Start() {
	ms.running = true
	go ms.run()
}

func (ms *multiSpinner) Stop() {
	if !ms.running {
		return
	}
	close(ms.stopCh)
	<-ms.doneCh
	ms.running = false

	// Clear spinner lines
	ms.clearLines()
}

func (ms *multiSpinner) SetStatus(idx int, status string, duration time.Duration) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.statuses[idx] = status
	ms.durations[idx] = duration
}

func (ms *multiSpinner) run() {
	defer close(ms.doneCh)

	ticker := time.NewTicker(ms.interval)
	defer ticker.Stop()

	ms.render()

	for {
		select {
		case <-ms.stopCh:
			return
		case <-ticker.C:
			ms.mu.Lock()
			ms.frameIdx = (ms.frameIdx + 1) % len(ms.frames)
			ms.mu.Unlock()
			ms.render()
		}
	}
}

func (ms *multiSpinner) render() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Move cursor up to overwrite previous render
	if ms.frameIdx > 0 {
		fmt.Fprintf(ms.writer, "\033[%dA", len(ms.tools))
	}

	frame := ms.frames[ms.frameIdx]

	for i, tool := range ms.tools {
		// Clear line
		fmt.Fprint(ms.writer, "\033[2K")

		status := ms.statuses[i]
		duration := ms.durations[i]

		switch status {
		case "running":
			fmt.Fprintf(ms.writer, "  %s %s\n", frame, tool.Name)
		case "ok":
			fmt.Fprintf(ms.writer, "  ✓ %s (%s)\n", tool.Name, formatDuration(duration))
		case "issues":
			fmt.Fprintf(ms.writer, "  ! %s (%s)\n", tool.Name, formatDuration(duration))
		case "error":
			fmt.Fprintf(ms.writer, "  ✗ %s (%s)\n", tool.Name, formatDuration(duration))
		}
	}
}

func (ms *multiSpinner) clearLines() {
	// Move up and clear each line
	for range ms.tools {
		fmt.Fprint(ms.writer, "\033[1A\033[2K")
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

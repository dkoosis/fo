package fo

import (
	"fmt"
	"os"
	"time"
)

// ProfileData tracks performance metrics for each pipeline stage.
type ProfileData struct {
	Stage          string        `json:"stage"`
	Duration       time.Duration `json:"duration"`
	DurationMs     int64         `json:"duration_ms"`
	PatternMatches int           `json:"pattern_matches,omitempty"`
	PatternSuccess int           `json:"pattern_success,omitempty"`
	BufferSize     int64         `json:"buffer_size,omitempty"`
	LineCount      int           `json:"line_count,omitempty"`
	MemoryAlloc    int64         `json:"memory_alloc,omitempty"`
}

// Profiler tracks performance metrics throughout command execution.
type Profiler struct {
	enabled   bool
	startTime time.Time
	stages    []ProfileData
	output    string // "stderr" or file path
}

// NewProfiler creates a new profiler instance.
func NewProfiler(enabled bool, output string) *Profiler {
	return &Profiler{
		enabled:   enabled,
		startTime: time.Now(),
		stages:    make([]ProfileData, 0),
		output:    output,
	}
}

// StartStage marks the start of a performance stage.
func (p *Profiler) StartStage(_ string) time.Time {
	if !p.enabled {
		return time.Now()
	}
	return time.Now()
}

// EndStage records the duration of a performance stage.
func (p *Profiler) EndStage(name string, start time.Time, metrics map[string]interface{}) {
	if !p.enabled {
		return
	}

	duration := time.Since(start)
	stage := ProfileData{
		Stage:      name,
		Duration:   duration,
		DurationMs: duration.Milliseconds(),
	}

	// Extract optional metrics
	if matches, ok := metrics["pattern_matches"].(int); ok {
		stage.PatternMatches = matches
	}
	if success, ok := metrics["pattern_success"].(int); ok {
		stage.PatternSuccess = success
	}
	if bufferSize, ok := metrics["buffer_size"].(int64); ok {
		stage.BufferSize = bufferSize
	}
	if lineCount, ok := metrics["line_count"].(int); ok {
		stage.LineCount = lineCount
	}
	if memAlloc, ok := metrics["memory_alloc"].(int64); ok {
		stage.MemoryAlloc = memAlloc
	}

	p.stages = append(p.stages, stage)
}

// Write outputs the profile data to the configured destination.
func (p *Profiler) Write() error {
	if !p.enabled || len(p.stages) == 0 {
		return nil
	}

	totalDuration := time.Since(p.startTime)
	output := "# fo Performance Profile\n"
	output += fmt.Sprintf("Total Duration: %s (%d ms)\n\n", totalDuration, totalDuration.Milliseconds())
	output += "Stage\tDuration\tDuration(ms)\tMatches\tSuccess\tBuffer\tLines\tMemory\n"
	output += "-----\t--------\t------------\t-------\t-------\t------\t-----\t------\n"

	for _, stage := range p.stages {
		output += fmt.Sprintf("%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
			stage.Stage,
			stage.Duration,
			stage.DurationMs,
			stage.PatternMatches,
			stage.PatternSuccess,
			stage.BufferSize,
			stage.LineCount,
			stage.MemoryAlloc,
		)
	}

	if p.output == "stderr" || p.output == "" {
		fmt.Fprintf(os.Stderr, "\n%s\n", output)
	} else {
		// Write to file
		return os.WriteFile(p.output, []byte(output), 0o600)
	}

	return nil
}

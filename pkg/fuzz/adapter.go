// Package fuzz provides parsing and rendering for Go fuzz testing output.
// Fuzz testing is built into Go 1.18+ and enabled with -fuzz flag.
package fuzz

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// FuzzStatus represents the overall status of a fuzz run.
type FuzzStatus string

const (
	StatusRunning FuzzStatus = "running"
	StatusPassed  FuzzStatus = "passed"
	StatusFailed  FuzzStatus = "failed"
)

// Progress represents a fuzz progress update.
type Progress struct {
	Elapsed        string
	Executions     int64
	ExecsPerSec    int64
	NewInteresting int
	TotalCorpus    int
	Workers        int
	Message        string // For baseline coverage messages
}

// Failure represents a fuzz test failure.
type Failure struct {
	TestName    string
	Duration    string
	Error       string
	CorpusFile  string
	RerunCmd    string
}

// Result represents parsed fuzz output.
type Result struct {
	Status    FuzzStatus
	Progress  []Progress
	Failures  []Failure
	TestName  string
}

// Adapter parses and renders fuzz output.
type Adapter struct {
	theme *design.Config
}

// NewAdapter creates a new adapter with the given theme.
func NewAdapter(theme *design.Config) *Adapter {
	return &Adapter{theme: theme}
}

// Patterns for parsing fuzz output
var (
	// Matches "fuzz: elapsed: 3s, execs: 102345 (34115/sec), new interesting: 12 (total: 22)"
	fuzzProgressRe = regexp.MustCompile(`fuzz:\s*elapsed:\s*(\S+),\s*execs:\s*(\d+)\s*\((\d+)/sec\),\s*new interesting:\s*(\d+)\s*\(total:\s*(\d+)\)`)
	// Matches "fuzz: elapsed: 0s, gathering baseline coverage: 10/10 completed, now fuzzing with 8 workers"
	fuzzBaselineRe = regexp.MustCompile(`fuzz:\s*elapsed:\s*(\S+),\s*(.+)`)
	// Matches "fuzz: elapsed: 0s, gathering baseline coverage: 0/10 completed"
	fuzzGatheringRe = regexp.MustCompile(`gathering baseline coverage:\s*(\d+)/(\d+)`)
	// Matches "now fuzzing with 8 workers"
	fuzzWorkersRe = regexp.MustCompile(`now fuzzing with (\d+) workers`)
	// Matches "--- FAIL: FuzzParseInput (0.52s)"
	fuzzFailRe = regexp.MustCompile(`---\s*FAIL:\s*(\S+)\s*\(([^)]+)\)`)
	// Matches "Failing input written to testdata/fuzz/FuzzParseInput/abc123def456"
	fuzzCorpusRe = regexp.MustCompile(`Failing input written to (\S+)`)
	// Matches "To re-run:" followed by command
	fuzzRerunRe = regexp.MustCompile(`go test -run=(\S+)`)
	// Matches panic or error lines
	panicRe = regexp.MustCompile(`panic:\s*(.+)`)
)

// Parse reads fuzz output from a reader.
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	result := &Result{
		Status: StatusRunning,
	}

	scanner := bufio.NewScanner(r)
	var currentFailure *Failure
	var lastError string

	for scanner.Scan() {
		line := scanner.Text()

		// Check for progress with execution stats
		if matches := fuzzProgressRe.FindStringSubmatch(line); matches != nil {
			execs, _ := strconv.ParseInt(matches[2], 10, 64)
			execsPerSec, _ := strconv.ParseInt(matches[3], 10, 64)
			newInteresting, _ := strconv.Atoi(matches[4])
			totalCorpus, _ := strconv.Atoi(matches[5])

			result.Progress = append(result.Progress, Progress{
				Elapsed:        matches[1],
				Executions:     execs,
				ExecsPerSec:    execsPerSec,
				NewInteresting: newInteresting,
				TotalCorpus:    totalCorpus,
			})
			continue
		}

		// Check for baseline/gathering progress
		if matches := fuzzBaselineRe.FindStringSubmatch(line); matches != nil {
			p := Progress{
				Elapsed: matches[1],
				Message: matches[2],
			}

			// Extract workers if present
			if workerMatches := fuzzWorkersRe.FindStringSubmatch(line); workerMatches != nil {
				p.Workers, _ = strconv.Atoi(workerMatches[1])
			}

			result.Progress = append(result.Progress, p)
			continue
		}

		// Check for test failure
		if matches := fuzzFailRe.FindStringSubmatch(line); matches != nil {
			if currentFailure != nil {
				result.Failures = append(result.Failures, *currentFailure)
			}
			currentFailure = &Failure{
				TestName: matches[1],
				Duration: matches[2],
			}
			if result.TestName == "" {
				result.TestName = matches[1]
			}
			result.Status = StatusFailed
			continue
		}

		// Check for panic/error
		if matches := panicRe.FindStringSubmatch(line); matches != nil {
			lastError = matches[1]
			if currentFailure != nil && currentFailure.Error == "" {
				currentFailure.Error = lastError
			}
			continue
		}

		// Check for corpus file
		if matches := fuzzCorpusRe.FindStringSubmatch(line); matches != nil {
			if currentFailure != nil {
				currentFailure.CorpusFile = matches[1]
			}
			continue
		}

		// Check for rerun command
		if matches := fuzzRerunRe.FindStringSubmatch(line); matches != nil {
			if currentFailure != nil {
				currentFailure.RerunCmd = "go test -run=" + matches[1]
			}
			continue
		}

		// Check for PASS
		if strings.HasPrefix(strings.TrimSpace(line), "PASS") {
			if result.Status != StatusFailed {
				result.Status = StatusPassed
			}
		}
	}

	// Add last failure if any
	if currentFailure != nil {
		result.Failures = append(result.Failures, *currentFailure)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// ParseBytes parses fuzz output from bytes.
func (a *Adapter) ParseBytes(data []byte) (*Result, error) {
	return a.Parse(bytes.NewReader(data))
}

// ParseString parses fuzz output from a string.
func (a *Adapter) ParseString(s string) (*Result, error) {
	return a.Parse(strings.NewReader(s))
}

// Render renders a fuzz result to a string.
func (a *Adapter) Render(result *Result) string {
	patterns := MapToPatterns(result)

	var sb strings.Builder
	for i, p := range patterns {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(p.Render(a.theme))
	}

	return sb.String()
}

// RenderReader reads and renders fuzz output from a reader.
func (a *Adapter) RenderReader(r io.Reader) (string, error) {
	result, err := a.Parse(r)
	if err != nil {
		return "", err
	}
	return a.Render(result), nil
}

// MapToPatterns converts fuzz result to fo patterns.
func MapToPatterns(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Summary pattern
	patterns = append(patterns, mapToSummary(r))

	// Failure table if there are any
	if len(r.Failures) > 0 {
		patterns = append(patterns, mapToFailureTable(r))
	}

	return patterns
}

func mapToSummary(r *Result) *design.Summary {
	// Get latest progress for stats
	var lastProgress *Progress
	if len(r.Progress) > 0 {
		lastProgress = &r.Progress[len(r.Progress)-1]
	}

	switch r.Status {
	case StatusPassed:
		metrics := []design.SummaryItem{
			{Label: "Status", Value: "OK", Type: "success"},
		}
		if lastProgress != nil {
			if lastProgress.Executions > 0 {
				metrics = append(metrics, design.SummaryItem{
					Label: "Executions",
					Value: formatNumber(lastProgress.Executions),
					Type:  "info",
				})
			}
			if lastProgress.TotalCorpus > 0 {
				metrics = append(metrics, design.SummaryItem{
					Label: "Corpus",
					Value: fmt.Sprintf("%d", lastProgress.TotalCorpus),
					Type:  "info",
				})
			}
		}
		return &design.Summary{
			Label:   "Fuzz Testing",
			Metrics: metrics,
		}

	case StatusFailed:
		metrics := []design.SummaryItem{
			{Label: "Status", Value: "FAIL", Type: "error"},
			{Label: "Crashes", Value: fmt.Sprintf("%d", len(r.Failures)), Type: "error"},
		}
		if lastProgress != nil && lastProgress.Executions > 0 {
			metrics = append(metrics, design.SummaryItem{
				Label: "Executions",
				Value: formatNumber(lastProgress.Executions),
				Type:  "info",
			})
		}
		return &design.Summary{
			Label:   "Fuzz Testing",
			Metrics: metrics,
		}

	default: // Running or unknown
		metrics := []design.SummaryItem{
			{Label: "Status", Value: "Running", Type: "info"},
		}
		if lastProgress != nil {
			if lastProgress.Executions > 0 {
				metrics = append(metrics, design.SummaryItem{
					Label: "Execs/sec",
					Value: formatNumber(lastProgress.ExecsPerSec),
					Type:  "info",
				})
			}
			if lastProgress.Message != "" {
				metrics = append(metrics, design.SummaryItem{
					Label: "Phase",
					Value: truncate(lastProgress.Message, 30),
					Type:  "info",
				})
			}
		}
		return &design.Summary{
			Label:   "Fuzz Testing",
			Metrics: metrics,
		}
	}
}

func mapToFailureTable(r *Result) *design.TestTable {
	items := make([]design.TestTableItem, 0, len(r.Failures))

	for _, f := range r.Failures {
		name := f.TestName

		// Build details
		var details string
		if f.Error != "" {
			details = f.Error
		}
		if f.CorpusFile != "" {
			if details != "" {
				details += " | "
			}
			// Show just the filename part of corpus
			if idx := strings.LastIndex(f.CorpusFile, "/"); idx != -1 {
				details += "corpus: " + f.CorpusFile[idx+1:]
			} else {
				details += "corpus: " + f.CorpusFile
			}
		}

		items = append(items, design.TestTableItem{
			Name:     name,
			Status:   "fail",
			Duration: f.Duration,
			Details:  details,
		})
	}

	density := design.DensityBalanced
	if len(items) > 10 {
		density = design.DensityCompact
	}

	return &design.TestTable{
		Label:   "Fuzz Crashes",
		Results: items,
		Density: density,
	}
}

func formatNumber(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// QuickRender renders fuzz output with default theme.
func QuickRender(output string) (string, error) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(output)
	if err != nil {
		return "", err
	}
	return adapter.Render(result), nil
}

// IsFuzzOutput detects if the data looks like fuzz output.
func IsFuzzOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	s := string(data)

	// Primary indicators: fuzz-specific patterns
	if strings.Contains(s, "fuzz: elapsed:") {
		return true
	}

	// Check for fuzz test failure pattern
	if fuzzFailRe.MatchString(s) && strings.Contains(s, "Fuzz") {
		return true
	}

	// Check for corpus file pattern
	if fuzzCorpusRe.MatchString(s) {
		return true
	}

	return false
}

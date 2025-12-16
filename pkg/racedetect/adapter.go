// Package racedetect provides parsing and rendering for Go race detector output.
// The race detector is built into Go and enabled with -race flag.
package racedetect

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

// AccessType represents the type of memory access in a race.
type AccessType string

const (
	AccessRead  AccessType = "read"
	AccessWrite AccessType = "write"
)

// Access represents a memory access in a data race.
type Access struct {
	Type       AccessType
	Address    string
	Goroutine  int
	Function   string
	File       string
	Line       int
	IsPrevious bool // true if this is the "Previous write/read" access
}

// GoroutineInfo represents goroutine creation info.
type GoroutineInfo struct {
	ID       int
	State    string
	Function string
	File     string
	Line     int
}

// Race represents a single data race.
type Race struct {
	Accesses   []Access
	Goroutines []GoroutineInfo
}

// Result represents parsed race detector output.
type Result struct {
	Races []Race
}

// Adapter parses and renders race detector output.
type Adapter struct {
	theme *design.Config
}

// NewAdapter creates a new adapter with the given theme.
func NewAdapter(theme *design.Config) *Adapter {
	return &Adapter{theme: theme}
}

// Patterns for parsing race detector output
var (
	// Matches "WARNING: DATA RACE"
	raceHeaderRe = regexp.MustCompile(`WARNING:\s*DATA\s*RACE`)
	// Matches "Read at 0x00c0001a4018 by goroutine 15:"
	readAtRe = regexp.MustCompile(`Read at (0x[0-9a-f]+) by goroutine (\d+):`)
	// Matches "Previous write at 0x00c0001a4018 by goroutine 7:"
	prevWriteRe = regexp.MustCompile(`Previous write at (0x[0-9a-f]+) by goroutine (\d+):`)
	// Matches "Write at 0x00c0001a4018 by goroutine 15:"
	writeAtRe = regexp.MustCompile(`Write at (0x[0-9a-f]+) by goroutine (\d+):`)
	// Matches "Previous read at 0x00c0001a4018 by goroutine 7:"
	prevReadRe = regexp.MustCompile(`Previous read at (0x[0-9a-f]+) by goroutine (\d+):`)
	// Matches "Goroutine 15 (running) created at:"
	goroutineCreatedRe = regexp.MustCompile(`Goroutine (\d+) \(([^)]+)\) created at:`)
	// Matches function calls like "  github.com/foo/bar.(*Server).handleRequest()"
	funcCallRe = regexp.MustCompile(`^\s+(\S+)\(`)
	// Matches file:line like "      /path/to/server.go:142 +0x1f4"
	fileLineRe = regexp.MustCompile(`^\s+(\S+):(\d+)`)
	// Race delimiter
	delimiterRe = regexp.MustCompile(`^={10,}$`)
)

// Parse reads race detector output from a reader.
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	var races []Race
	scanner := bufio.NewScanner(r)

	var currentRace *Race
	var inRace bool
	var lastAccess *Access
	var lastGoroutine *GoroutineInfo
	var expectingFileLine bool
	var expectingGoroutineFileLine bool

	for scanner.Scan() {
		line := scanner.Text()

		// Check for race delimiter
		if delimiterRe.MatchString(strings.TrimSpace(line)) {
			if inRace && currentRace != nil {
				races = append(races, *currentRace)
				currentRace = nil
			}
			inRace = false
			continue
		}

		// Check for race header
		if raceHeaderRe.MatchString(line) {
			currentRace = &Race{}
			inRace = true
			continue
		}

		if !inRace || currentRace == nil {
			continue
		}

		// Check for read access
		if matches := readAtRe.FindStringSubmatch(line); matches != nil {
			goroutine, _ := strconv.Atoi(matches[2])
			access := Access{
				Type:      AccessRead,
				Address:   matches[1],
				Goroutine: goroutine,
			}
			currentRace.Accesses = append(currentRace.Accesses, access)
			lastAccess = &currentRace.Accesses[len(currentRace.Accesses)-1]
			lastGoroutine = nil
			expectingFileLine = true
			expectingGoroutineFileLine = false
			continue
		}

		// Check for write access
		if matches := writeAtRe.FindStringSubmatch(line); matches != nil {
			goroutine, _ := strconv.Atoi(matches[2])
			access := Access{
				Type:      AccessWrite,
				Address:   matches[1],
				Goroutine: goroutine,
			}
			currentRace.Accesses = append(currentRace.Accesses, access)
			lastAccess = &currentRace.Accesses[len(currentRace.Accesses)-1]
			lastGoroutine = nil
			expectingFileLine = true
			expectingGoroutineFileLine = false
			continue
		}

		// Check for previous write access
		if matches := prevWriteRe.FindStringSubmatch(line); matches != nil {
			goroutine, _ := strconv.Atoi(matches[2])
			access := Access{
				Type:       AccessWrite,
				Address:    matches[1],
				Goroutine:  goroutine,
				IsPrevious: true,
			}
			currentRace.Accesses = append(currentRace.Accesses, access)
			lastAccess = &currentRace.Accesses[len(currentRace.Accesses)-1]
			lastGoroutine = nil
			expectingFileLine = true
			expectingGoroutineFileLine = false
			continue
		}

		// Check for previous read access
		if matches := prevReadRe.FindStringSubmatch(line); matches != nil {
			goroutine, _ := strconv.Atoi(matches[2])
			access := Access{
				Type:       AccessRead,
				Address:    matches[1],
				Goroutine:  goroutine,
				IsPrevious: true,
			}
			currentRace.Accesses = append(currentRace.Accesses, access)
			lastAccess = &currentRace.Accesses[len(currentRace.Accesses)-1]
			lastGoroutine = nil
			expectingFileLine = true
			expectingGoroutineFileLine = false
			continue
		}

		// Check for goroutine creation
		if matches := goroutineCreatedRe.FindStringSubmatch(line); matches != nil {
			id, _ := strconv.Atoi(matches[1])
			gi := GoroutineInfo{
				ID:    id,
				State: matches[2],
			}
			currentRace.Goroutines = append(currentRace.Goroutines, gi)
			lastGoroutine = &currentRace.Goroutines[len(currentRace.Goroutines)-1]
			lastAccess = nil
			expectingFileLine = false
			expectingGoroutineFileLine = true
			continue
		}

		// Extract function name
		if matches := funcCallRe.FindStringSubmatch(line); matches != nil {
			if lastAccess != nil && lastAccess.Function == "" {
				lastAccess.Function = matches[1]
			} else if lastGoroutine != nil && lastGoroutine.Function == "" {
				lastGoroutine.Function = matches[1]
			}
			continue
		}

		// Extract file:line
		if matches := fileLineRe.FindStringSubmatch(line); matches != nil {
			lineNum, _ := strconv.Atoi(matches[2])
			if expectingFileLine && lastAccess != nil && lastAccess.File == "" {
				lastAccess.File = matches[1]
				lastAccess.Line = lineNum
				expectingFileLine = false
			} else if expectingGoroutineFileLine && lastGoroutine != nil && lastGoroutine.File == "" {
				lastGoroutine.File = matches[1]
				lastGoroutine.Line = lineNum
				expectingGoroutineFileLine = false
			}
		}
	}

	// Handle last race if not closed by delimiter
	if currentRace != nil && len(currentRace.Accesses) > 0 {
		races = append(races, *currentRace)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Result{Races: races}, nil
}

// ParseBytes parses race detector output from bytes.
func (a *Adapter) ParseBytes(data []byte) (*Result, error) {
	return a.Parse(bytes.NewReader(data))
}

// ParseString parses race detector output from a string.
func (a *Adapter) ParseString(s string) (*Result, error) {
	return a.Parse(strings.NewReader(s))
}

// Render renders a race detector result to a string.
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

// RenderReader reads and renders race detector output from a reader.
func (a *Adapter) RenderReader(r io.Reader) (string, error) {
	result, err := a.Parse(r)
	if err != nil {
		return "", err
	}
	return a.Render(result), nil
}

// MapToPatterns converts race detector result to fo patterns.
func MapToPatterns(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Summary pattern
	patterns = append(patterns, mapToSummary(r))

	// Race table if there are any
	if len(r.Races) > 0 {
		patterns = append(patterns, mapToRaceTable(r))
	}

	return patterns
}

func mapToSummary(r *Result) *design.Summary {
	if len(r.Races) == 0 {
		return &design.Summary{
			Label: "Race Detection",
			Metrics: []design.SummaryItem{
				{Label: "Status", Value: "OK", Type: "success"},
				{Label: "Races", Value: "None", Type: "info"},
			},
		}
	}

	return &design.Summary{
		Label: "Race Detection",
		Metrics: []design.SummaryItem{
			{Label: "Status", Value: "FAIL", Type: "error"},
			{Label: "Races", Value: fmt.Sprintf("%d", len(r.Races)), Type: "error"},
		},
	}
}

func mapToRaceTable(r *Result) *design.TestTable {
	items := make([]design.TestTableItem, 0, len(r.Races))

	for i, race := range r.Races {
		name := fmt.Sprintf("Race #%d", i+1)

		// Build details from accesses
		var details []string
		for _, access := range race.Accesses {
			prefix := ""
			if access.IsPrevious {
				prefix = "prev "
			}
			accessType := string(access.Type)

			// Shorten function name
			fn := access.Function
			if idx := strings.LastIndex(fn, "/"); idx != -1 {
				fn = fn[idx+1:]
			}

			if fn != "" {
				details = append(details, fmt.Sprintf("%s%s in %s", prefix, accessType, fn))
			} else if access.File != "" {
				details = append(details, fmt.Sprintf("%s%s at %s:%d", prefix, accessType, shortenPath(access.File), access.Line))
			}
		}

		items = append(items, design.TestTableItem{
			Name:    name,
			Status:  "fail",
			Details: strings.Join(details, " vs "),
		})
	}

	density := design.DensityBalanced
	if len(items) > 10 {
		density = design.DensityCompact
	}

	return &design.TestTable{
		Label:   "Data Races",
		Results: items,
		Density: density,
	}
}

func shortenPath(path string) string {
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		return path[idx+1:]
	}
	return path
}

// QuickRender renders race detector output with default theme.
func QuickRender(output string) (string, error) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(output)
	if err != nil {
		return "", err
	}
	return adapter.Render(result), nil
}

// IsRaceDetectorOutput detects if the data looks like race detector output.
func IsRaceDetectorOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	s := string(data)

	// Primary indicator: race detector's specific warning
	return raceHeaderRe.MatchString(s)
}

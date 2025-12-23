package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RaceFormatter handles go test -race output.
type RaceFormatter struct{}

func (f *RaceFormatter) Matches(command string) bool {
	return strings.Contains(command, "go test") && strings.Contains(command, "-race")
}

func (f *RaceFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	s := Styles()
	// Non-bold variants for race output details
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6"))
	funcStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))

	// Parse JSON events and extract race warnings
	var races []string
	var testsPassed, testsFailed, testsSkipped int
	var raceDetected bool

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event GoTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not JSON - parse verbose output format
			parseVerboseLine(line, &testsPassed, &testsFailed, &testsSkipped, &raceDetected, &races)
			continue
		}

		// Check for race warnings in output
		if strings.Contains(event.Output, "WARNING: DATA RACE") {
			raceDetected = true
		}

		// Collect race-related output
		if event.Action == actionOutput && event.Test == "" {
			collectRaceOutput(event.Output, &races)
		}

		// Track test results
		if event.Action == "pass" && event.Test != "" {
			testsPassed++
		} else if event.Action == "fail" && event.Test != "" {
			testsFailed++
		} else if event.Action == "skip" && event.Test != "" {
			testsSkipped++
		}
	}

	// Build summary
	if raceDetected {
		b.WriteString(s.Error.Render("✗ DATA RACE DETECTED"))
		b.WriteString("\n\n")

		// Show race details
		for i, line := range races {
			if i >= 30 { // Limit output
				b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more lines\n", len(races)-30)))
				break
			}

			// Style different parts of race output
			if strings.Contains(line, "DATA RACE") {
				b.WriteString(s.Warn.Render(line))
			} else if strings.Contains(line, ".go:") {
				b.WriteString(fileStyle.Render(line))
			} else if strings.Contains(line, "Read at") || strings.Contains(line, "Write at") ||
				strings.Contains(line, "Previous") {
				b.WriteString(funcStyle.Render(line))
			} else {
				b.WriteString(s.Muted.Render(line))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString(s.Success.Render("✓ No data races detected"))
		b.WriteString("\n")
	}

	// Show test summary
	if testsPassed+testsFailed+testsSkipped > 0 {
		b.WriteString("\n")
		total := testsPassed + testsFailed + testsSkipped
		if testsFailed > 0 {
			b.WriteString(s.Error.Render(fmt.Sprintf("Tests: %d passed, %d failed", testsPassed, testsFailed)))
		} else {
			b.WriteString(s.Success.Render(fmt.Sprintf("Tests: %d passed", testsPassed)))
		}
		if testsSkipped > 0 {
			b.WriteString(s.Muted.Render(fmt.Sprintf(", %d skipped", testsSkipped)))
		}
		b.WriteString(s.Muted.Render(fmt.Sprintf(" (total: %d)", total)))
		b.WriteString("\n")
	}

	return b.String()
}

// parseVerboseLine parses go test -v output format (non-JSON).
func parseVerboseLine(line string, passed, failed, skipped *int, raceDetected *bool, races *[]string) {
	// Check for race warning
	if strings.Contains(line, "WARNING: DATA RACE") {
		*raceDetected = true
	}

	// Collect race-related output
	collectRaceOutput(line, races)

	// Parse test results: "--- PASS: TestName (0.00s)"
	if strings.HasPrefix(line, "--- PASS:") {
		*passed++
	} else if strings.HasPrefix(line, "--- FAIL:") {
		*failed++
	} else if strings.HasPrefix(line, "--- SKIP:") {
		*skipped++
	}
}

// collectRaceOutput appends race-related lines to the races slice.
func collectRaceOutput(output string, races *[]string) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return
	}
	if strings.Contains(trimmed, "DATA RACE") ||
		strings.Contains(trimmed, "Read at") ||
		strings.Contains(trimmed, "Write at") ||
		strings.Contains(trimmed, "Previous write") ||
		strings.Contains(trimmed, "Previous read") ||
		strings.Contains(trimmed, "Goroutine") ||
		(strings.HasPrefix(trimmed, "  ") && strings.Contains(trimmed, ".go:")) {
		*races = append(*races, trimmed)
	}
}

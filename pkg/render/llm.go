package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/pattern"
)

const statusFail = "fail"

// LLM renders patterns as terse plain text optimized for AI consumption.
// Zero ANSI codes, deterministic sort, SCOPE line, importance-budgeted truncation.
type LLM struct{}

// NewLLM creates an LLM renderer.
func NewLLM() *LLM {
	return &LLM{}
}

// Render formats all patterns for LLM consumption.
func (l *LLM) Render(patterns []pattern.Pattern) string {
	// Separate by type to build appropriate output
	var summaries []*pattern.Summary
	var tables []*pattern.TestTable

	for _, p := range patterns {
		switch v := p.(type) {
		case *pattern.Summary:
			summaries = append(summaries, v)
		case *pattern.TestTable:
			tables = append(tables, v)
		}
	}

	// TODO: replace string-prefix dispatch with Summary.Kind field
	isReport := false
	isTestOutput := false
	for _, s := range summaries {
		if strings.HasPrefix(s.Label, "REPORT:") {
			isReport = true
			break
		}
		if strings.HasPrefix(s.Label, "PASS") || strings.HasPrefix(s.Label, "FAIL") {
			isTestOutput = true
		}
	}

	if isReport {
		return l.renderReport(summaries, tables)
	}
	if isTestOutput {
		return l.renderTestOutput(summaries, tables)
	}
	return l.renderSARIFOutput(tables)
}

func (l *LLM) renderReport(summaries []*pattern.Summary, tables []*pattern.TestTable) string {
	var sb strings.Builder

	var reportSummary *pattern.Summary
	for _, s := range summaries {
		if strings.HasPrefix(s.Label, "REPORT:") {
			reportSummary = s
			break
		}
	}
	if reportSummary == nil {
		return ""
	}

	sb.WriteString(reportSummary.Label + "\n")

	// Build a map of tables by tool name prefix
	tablesByTool := make(map[string][]*pattern.TestTable)
	for _, t := range tables {
		for _, m := range reportSummary.Metrics {
			if strings.HasPrefix(t.Label, m.Label) {
				tablesByTool[m.Label] = append(tablesByTool[m.Label], t)
			}
		}
	}

	for _, m := range reportSummary.Metrics {
		sb.WriteString("\n" + m.Label + ": " + m.Value + "\n")

		for _, t := range tablesByTool[m.Label] {
			sb.WriteString("\n")
			for _, item := range t.Results {
				prefix := "  "
				if item.Status == statusFail {
					prefix = "  FAIL "
				}
				sb.WriteString(prefix + item.Name)
				if item.Duration != "" {
					sb.WriteString(" (" + item.Duration + ")")
				}
				sb.WriteString("\n")
				if item.Details != "" {
					lines := strings.Split(item.Details, "\n")
					max := 3
					if len(lines) < max {
						max = len(lines)
					}
					for _, line := range lines[:max] {
						sb.WriteString("    " + line + "\n")
					}
					if len(lines) > 3 {
						sb.WriteString(fmt.Sprintf("    ... (%d more lines)\n", len(lines)-3))
					}
				}
			}
		}
	}

	return sb.String()
}

func (l *LLM) renderSARIFOutput(tables []*pattern.TestTable) string {
	var sb strings.Builder

	// SCOPE line
	scope := sarifScope(tables)
	sb.WriteString("SCOPE: " + scope + "\n")

	// Collect all items across tables, grouped by file (table label = file path)
	type diagEntry struct {
		file    string
		level   string
		rule    string
		line    int
		col     int
		message string
	}

	diags := make([]diagEntry, 0, len(tables)*4)
	for _, t := range tables {
		for _, item := range t.Results {
			var level string
			switch item.Status {
			case statusFail:
				level = "ERR"
			case "pass":
				level = "NOTE"
			default:
				level = "WARN"
			}

			rule, line, col := parseRuleLocation(item.Name)
			diags = append(diags, diagEntry{
				file:    t.Label,
				level:   level,
				rule:    rule,
				line:    line,
				col:     col,
				message: item.Details,
			})
		}
	}

	// Sort: severity desc → file asc → line asc → rule asc
	sort.Slice(diags, func(i, j int) bool {
		pi, pj := llmLevelPriority(diags[i].level), llmLevelPriority(diags[j].level)
		if pi != pj {
			return pi < pj
		}
		if diags[i].file != diags[j].file {
			return diags[i].file < diags[j].file
		}
		if diags[i].line != diags[j].line {
			return diags[i].line < diags[j].line
		}
		return diags[i].rule < diags[j].rule
	})

	// Group by file and render
	currentFile := ""
	for _, d := range diags {
		if d.file != currentFile {
			currentFile = d.file
			sb.WriteString("\n## " + d.file + "\n")
		}
		if d.line > 0 {
			sb.WriteString(fmt.Sprintf("  %s %s:%d:%d %s\n", d.level, d.rule, d.line, d.col, d.message))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", d.level, d.rule, d.message))
		}
	}

	return sb.String()
}

func (l *LLM) renderTestOutput(summaries []*pattern.Summary, tables []*pattern.TestTable) string {
	var sb strings.Builder

	// SCOPE line from summary
	for _, s := range summaries {
		sb.WriteString("SCOPE: " + s.Label + "\n")
	}

	// Render tables in order (panics → build errors → failures → passes)
	for _, t := range tables {
		sb.WriteString("\n")
		sb.WriteString(t.Label + "\n")
		for _, item := range t.Results {
			prefix := "  PASS"
			switch item.Status {
			case statusFail:
				prefix = "  FAIL"
			case "skip":
				prefix = "  SKIP"
			}

			dur := ""
			if item.Duration != "" {
				dur = " (" + item.Duration + ")"
			}
			sb.WriteString(fmt.Sprintf("%s %s%s\n", prefix, item.Name, dur))

			if item.Details != "" {
				lines := strings.Split(item.Details, "\n")
				max := 3
				if len(lines) < max {
					max = len(lines)
				}
				for _, line := range lines[:max] {
					sb.WriteString("    " + line + "\n")
				}
				if len(lines) > 3 {
					sb.WriteString(fmt.Sprintf("    ... (%d more lines)\n", len(lines)-3))
				}
			}
		}
	}

	return sb.String()
}

func sarifScope(tables []*pattern.TestTable) string {
	fileCount := len(tables)
	var errCount, warnCount, noteCount int
	for _, t := range tables {
		for _, item := range t.Results {
			switch item.Status {
			case statusFail:
				errCount++
			case "skip":
				warnCount++
			default:
				noteCount++
			}
		}
	}
	total := errCount + warnCount + noteCount

	parts := []string{fmt.Sprintf("%d files", fileCount), fmt.Sprintf("%d diags", total)}
	var breakdown []string
	if errCount > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d err", errCount))
	}
	if warnCount > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d warn", warnCount))
	}
	if noteCount > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d note", noteCount))
	}
	if len(breakdown) > 0 {
		parts = append(parts, "("+strings.Join(breakdown, ", ")+")")
	}
	return strings.Join(parts, ", ")
}

// parseRuleLocation splits "ruleId:line:col" into components.
func parseRuleLocation(name string) (rule string, line, col int) {
	parts := strings.Split(name, ":")
	if len(parts) >= 3 {
		rule = parts[0]
		line, _ = strconv.Atoi(parts[1])
		col, _ = strconv.Atoi(parts[2])
		return rule, line, col
	}
	if len(parts) >= 2 {
		rule = parts[0]
		line, _ = strconv.Atoi(parts[1])
		return rule, line, 0
	}
	return name, 0, 0
}

func llmLevelPriority(level string) int {
	switch level {
	case "ERR":
		return 0
	case "WARN":
		return 1
	default:
		return 2
	}
}

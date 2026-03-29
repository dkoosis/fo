package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/pattern"
)

const (
	formatVersion  = "fo:llm:v1"
	maxDetailLines = 5
	symError       = "✗"
	symWarning     = "⚠"
	symNote        = "ℹ"
	symPass        = "✔"
)

// LLM renders patterns as terse plain text optimized for LLM consumption.
// Zero ANSI codes, deterministic sort, severity-first, action-oriented.
type LLM struct{}

// NewLLM creates an LLM renderer.
func NewLLM() *LLM {
	return &LLM{}
}

// severitySymbol maps a SARIF-context Status to a severity symbol.
// ONLY use for SARIF-mode items where StatusFail=error, StatusSkip=warning, StatusPass=note.
// For test-mode items, always use symError for failures and drop passes/skips.
func severitySymbol(status pattern.Status) string {
	switch status {
	case pattern.StatusFail:
		return symError
	case pattern.StatusSkip:
		return symWarning
	case pattern.StatusPass:
		return symNote
	default:
		return symNote
	}
}

// severityPriority returns sort priority (lower = more severe).
func severityPriority(sym string) int {
	switch sym {
	case symError:
		return 0
	case symWarning:
		return 1
	default:
		return 2
	}
}

// isSARIFTable returns true if the table contains SARIF-style rule:line:col items.
func isSARIFTable(t *pattern.TestTable) bool {
	for _, item := range t.Results {
		_, line, _ := parseRuleLocation(item.Name)
		if line > 0 {
			return true
		}
	}
	for _, item := range t.Results {
		if strings.Contains(item.Name, ":") {
			return true
		}
	}
	return false
}

// Render formats all patterns for LLM consumption.
func (l *LLM) Render(patterns []pattern.Pattern) string {
	var sb strings.Builder
	sb.WriteString(formatVersion + "\n\n")

	var summaries []*pattern.Summary
	var tables []*pattern.TestTable
	var errors []*pattern.Error

	for _, p := range patterns {
		switch v := p.(type) {
		case *pattern.Summary:
			summaries = append(summaries, v)
		case *pattern.TestTable:
			tables = append(tables, v)
		case *pattern.Error:
			errors = append(errors, v)
		}
	}

	if len(summaries) > 0 {
		switch summaries[0].Kind {
		case pattern.SummaryKindReport:
			sb.WriteString(l.renderReport(summaries, tables, errors))
		case pattern.SummaryKindTest:
			sb.WriteString(l.renderTestOutput(summaries, tables))
		case pattern.SummaryKindSARIF:
			sb.WriteString(l.renderSARIFOutput(tables))
		default:
			sb.WriteString(l.renderSARIFOutput(tables))
		}
	} else {
		sb.WriteString(l.renderSARIFOutput(tables))
	}

	return sb.String()
}

func (l *LLM) renderReport(summaries []*pattern.Summary, tables []*pattern.TestTable, errors []*pattern.Error) string {
	var sb strings.Builder

	// Caller dispatches here only when summaries[0].Kind == SummaryKindReport.
	reportSummary := summaries[0]
	sb.WriteString(reportSummary.Label + "\n")

	// Group tables by originating tool
	tablesByTool := make(map[string][]*pattern.TestTable)
	for _, t := range tables {
		if t.Source != "" {
			tablesByTool[t.Source] = append(tablesByTool[t.Source], t)
		}
	}
	errorsByTool := make(map[string][]*pattern.Error)
	for _, e := range errors {
		errorsByTool[e.Source] = append(errorsByTool[e.Source], e)
	}

	for _, m := range reportSummary.Metrics {
		sb.WriteString("\n" + m.Label + ": " + m.Value + "\n")

		// Render parse errors for this tool
		for _, e := range errorsByTool[m.Label] {
			sb.WriteString("  ERROR: " + e.Message + "\n")
		}

		for _, t := range tablesByTool[m.Label] {
			if t.Label != "" {
				sb.WriteString("\n## " + t.Label + "\n")
			} else {
				sb.WriteString("\n")
			}
			for _, item := range t.Results {
				prefix := "  "
				if item.Status == pattern.StatusFail {
					prefix = "  FAIL "
				}
				sb.WriteString(prefix + item.Name)
				if item.Duration != "" {
					sb.WriteString(" (" + item.Duration + ")")
				}
				sb.WriteString("\n")
				writeDetails(&sb, item.Details)
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
			case pattern.StatusFail:
				level = "ERR"
			case pattern.StatusPass:
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
			fmt.Fprintf(&sb, "  %s %s:%d:%d %s\n", d.level, d.rule, d.line, d.col, d.message)
		} else {
			fmt.Fprintf(&sb, "  %s %s %s\n", d.level, d.rule, d.message)
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
			case pattern.StatusFail:
				prefix = "  FAIL"
			case pattern.StatusSkip:
				prefix = "  SKIP"
			}

			dur := ""
			if item.Duration != "" {
				dur = " (" + item.Duration + ")"
			}
			fmt.Fprintf(&sb, "%s %s%s\n", prefix, item.Name, dur)

			writeDetails(&sb, item.Details)
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
			case pattern.StatusFail:
				errCount++
			case pattern.StatusSkip:
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
// Note: splits left-to-right on ':', so colons in paths (e.g. Windows drive letters)
// would mis-parse. Currently safe — SARIF URIs use POSIX convention and the mapper
// stores only the rule:line:col portion in TestTableItem.Name.
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

// writeDetails appends a truncated detail block (max 3 lines, 4-space indent).
func writeDetails(sb *strings.Builder, details string) {
	if details == "" {
		return
	}
	lines := strings.Split(details, "\n")
	show := 3
	if len(lines) < show {
		show = len(lines)
	}
	for _, line := range lines[:show] {
		sb.WriteString("    " + line + "\n")
	}
	if len(lines) > 3 {
		fmt.Fprintf(sb, "    ... (%d more lines)\n", len(lines)-3)
	}
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

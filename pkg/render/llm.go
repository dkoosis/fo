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
type LLM struct {
	meta RunMeta
}

// NewLLM creates an LLM renderer.
func NewLLM() *LLM {
	return &LLM{}
}

// WithMeta attaches run envelope metadata for rendering.
func (l *LLM) WithMeta(m RunMeta) *LLM {
	l.meta = m
	return l
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
	return false
}

// hasActionableContent returns true if the tool's tables or errors contain
// anything worth rendering (SARIF diags, test failures, or error patterns).
func hasActionableContent(tables []*pattern.TestTable, errors []*pattern.Error) bool {
	if len(errors) > 0 {
		return true
	}
	for _, t := range tables {
		if isSARIFTable(t) {
			return true
		}
		for _, item := range t.Results {
			if item.Status == pattern.StatusFail {
				return true
			}
		}
	}
	return false
}

// Render formats all patterns for LLM consumption.
func (l *LLM) Render(patterns []pattern.Pattern) string {
	var sb strings.Builder
	sb.WriteString(formatVersion + "\n")
	if l.meta.DataHash != "" {
		sb.WriteString("data_hash: " + l.meta.DataHash + "\n")
	}
	if l.meta.GeneratedAt != "" {
		sb.WriteString("generated_at: " + l.meta.GeneratedAt + "\n")
	}
	sb.WriteString("\n")

	// Check for pattern types that use direct type-switch rendering
	// (newer pattern types that don't fit the Summary/TestTable dispatch).
	for _, p := range patterns {
		if s := l.renderOne(p); s != "" {
			sb.WriteString(s)
			return sb.String()
		}
	}

	// Legacy dispatch: collect summaries/tables/errors, dispatch by SummaryKind.
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

// renderOne handles pattern types via direct type switch.
// Returns "" for pattern types handled by the legacy SummaryKind dispatch.
func (l *LLM) renderOne(p pattern.Pattern) string {
	switch v := p.(type) {
	case *pattern.JTBDCoverage:
		return l.renderJTBDCoverage(v)
	default:
		return ""
	}
}

func (l *LLM) renderJTBDCoverage(j *pattern.JTBDCoverage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "JTBD Coverage  %d/%d jobs covered\n\n", j.CoveredJobs, j.TotalJobs)

	// Header
	sb.WriteString(" Code      Tests  Pass  Fail  Job\n")

	for _, e := range j.Entries {
		icon := "·"
		if e.TestCount > 0 && e.Fail == 0 {
			icon = "✓"
		} else if e.Fail > 0 {
			icon = "✗"
		}

		passStr := fmt.Sprintf("%d", e.Pass)
		failStr := fmt.Sprintf("%d", e.Fail)
		if e.TestCount == 0 {
			passStr = "—"
			failStr = "—"
		}

		name := e.Name
		if name != "" {
			name = icon + " " + name
		}

		fmt.Fprintf(&sb, " %-8s  %4d  %4s  %4s  %s\n",
			e.Code, e.TestCount, passStr, failStr, name)
	}

	return sb.String()
}

// classifyTools counts findings per severity and splits tools into failing/passing.
func classifyTools(
	metrics []pattern.SummaryItem,
	tablesByTool map[string][]*pattern.TestTable,
	errorsByTool map[string][]*pattern.Error,
) (errTotal, warnTotal, noteTotal int, failingTools, passingTools []string) {
	for _, m := range metrics {
		tool := m.Label
		toolErrors := len(errorsByTool[tool])

		for _, t := range tablesByTool[tool] {
			if isSARIFTable(t) {
				for _, item := range t.Results {
					sym := severitySymbol(item.Status)
					switch sym {
					case symError:
						errTotal++
						toolErrors++
					case symWarning:
						warnTotal++
					default:
						noteTotal++
					}
				}
			} else {
				for _, item := range t.Results {
					if item.Status == pattern.StatusFail {
						errTotal++
						toolErrors++
					}
				}
			}
		}

		errTotal += len(errorsByTool[tool])

		if toolErrors > 0 {
			failingTools = append(failingTools, tool)
		} else {
			passingTools = append(passingTools, tool)
		}
	}
	return errTotal, warnTotal, noteTotal, failingTools, passingTools
}

func (l *LLM) renderReport(summaries []*pattern.Summary, tables []*pattern.TestTable, errors []*pattern.Error) string {
	var sb strings.Builder

	reportSummary := summaries[0]

	// Group tables and errors by tool
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

	// Count findings and classify tools
	errTotal, warnTotal, noteTotal, failingTools, passingTools := classifyTools(
		reportSummary.Metrics, tablesByTool, errorsByTool,
	)

	// Triage line — guard empty groups
	sb.WriteString(triageCounts(errTotal, warnTotal, noteTotal))
	if len(failingTools) > 0 {
		sb.WriteString(" | " + strings.Join(failingTools, " "))
	}
	if len(passingTools) > 0 {
		sb.WriteString(" | " + strings.Join(passingTools, " ") + " " + symPass)
	}
	sb.WriteString("\n")

	// Tool sections — only tools with findings, in report delimiter order
	for _, m := range reportSummary.Metrics {
		tool := m.Label
		toolTables := tablesByTool[tool]
		toolErrors := errorsByTool[tool]

		if !hasActionableContent(toolTables, toolErrors) {
			continue
		}

		sb.WriteString("\n## " + tool + "\n")

		// Error patterns first
		for _, e := range toolErrors {
			sb.WriteString("  " + symError + " " + e.Message + "\n")
		}

		// Branch: SARIF vs test rendering
		hasSARIF := false
		for _, t := range toolTables {
			if isSARIFTable(t) {
				hasSARIF = true
				break
			}
		}

		if hasSARIF {
			diags, _, _, _ := collectSARIFDiags(toolTables)
			sortDiags(diags)
			renderDiags(&sb, diags)
		} else {
			for _, t := range toolTables {
				for _, item := range t.Results {
					if item.Status != pattern.StatusFail {
						continue
					}
					pkg := extractPackage(t.Label)
					sb.WriteString("  " + symError + " " + pkg + " " + item.Name)
					if item.Duration != "" {
						sb.WriteString(" (" + item.Duration + ")")
					}
					sb.WriteString("\n")
					writeDetails(&sb, item.Details)
					writeFixCommand(&sb, item.FixCommand)
				}
			}
		}
	}

	return sb.String()
}

// diagEntry represents a single diagnostic finding for LLM rendering.
type diagEntry struct {
	sym        string
	file       string
	rule       string
	line       int
	col        int
	message    string
	fixCommand string
}

// collectSARIFDiags extracts diagnostic entries from SARIF-mode tables.
func collectSARIFDiags(tables []*pattern.TestTable) ([]diagEntry, int, int, int) {
	var diags []diagEntry
	var errCount, warnCount, noteCount int
	for _, t := range tables {
		for _, item := range t.Results {
			sym := severitySymbol(item.Status)
			switch sym {
			case symError:
				errCount++
			case symWarning:
				warnCount++
			default:
				noteCount++
			}
			rule, line, col := parseRuleLocation(item.Name)
			diags = append(diags, diagEntry{
				sym: sym, file: t.Label, rule: rule,
				line: line, col: col, message: item.Details,
				fixCommand: item.FixCommand,
			})
		}
	}
	return diags, errCount, warnCount, noteCount
}

// sortDiags sorts diagnostics by severity desc → file asc → line asc → rule asc.
func sortDiags(diags []diagEntry) {
	sort.Slice(diags, func(i, j int) bool {
		pi, pj := severityPriority(diags[i].sym), severityPriority(diags[j].sym)
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
}

// renderDiags writes sorted diagnostic lines to a builder.
func renderDiags(sb *strings.Builder, diags []diagEntry) {
	for _, d := range diags {
		sb.WriteString("  " + d.sym + " ")
		if d.line > 0 {
			fmt.Fprintf(sb, "%s:%d:%d %s", d.file, d.line, d.col, d.rule)
		} else {
			sb.WriteString(d.file + " " + d.rule)
		}
		if d.message != "" {
			sb.WriteString(" — " + d.message)
		}
		sb.WriteString("\n")
		writeFixCommand(sb, d.fixCommand)
	}
}

// writeFixCommand renders a non-empty FixCommand as a fenced bash block.
// Skips silently when empty so callers don't emit empty fences.
func writeFixCommand(sb *strings.Builder, cmd string) {
	if cmd == "" {
		return
	}
	sb.WriteString("    ```bash\n")
	sb.WriteString("    " + cmd + "\n")
	sb.WriteString("    ```\n")
}

// triageCounts formats the triage count string, omitting ℹ when zero.
func triageCounts(errCount, warnCount, noteCount int) string {
	s := fmt.Sprintf("%d %s %d %s", errCount, symError, warnCount, symWarning)
	if noteCount > 0 {
		s += fmt.Sprintf(" %d %s", noteCount, symNote)
	}
	return s
}

func (l *LLM) renderSARIFOutput(tables []*pattern.TestTable) string {
	var sb strings.Builder

	diags, errCount, warnCount, noteCount := collectSARIFDiags(tables)
	sb.WriteString(triageCounts(errCount, warnCount, noteCount) + "\n")

	if len(diags) == 0 {
		return sb.String()
	}

	sortDiags(diags)
	sb.WriteString("\n")
	renderDiags(&sb, diags)

	return sb.String()
}

func (l *LLM) renderTestOutput(summaries []*pattern.Summary, tables []*pattern.TestTable) string {
	var sb strings.Builder

	// Count failures and totals from summary metrics (reliable source)
	failCount, totalTests, pkgCount := 0, 0, 0
	duration := ""
	if len(summaries) > 0 {
		for _, m := range summaries[0].Metrics {
			switch m.Label {
			case "Failed":
				_, _ = fmt.Sscanf(m.Value, "%d/", &failCount)
			case "Passed":
				_, _ = fmt.Sscanf(m.Value, "%d/%d", new(int), &totalTests)
			case "Packages":
				_, _ = fmt.Sscanf(m.Value, "%d", &pkgCount)
			case "Panics":
				n := 0
				_, _ = fmt.Sscanf(m.Value, "%d", &n)
				failCount += n
			case "Build Errors":
				n := 0
				_, _ = fmt.Sscanf(m.Value, "%d", &n)
				failCount += n
			}
		}
		// Extract duration from summary label
		label := summaries[0].Label
		if idx := strings.LastIndex(label, "("); idx >= 0 {
			if end := strings.Index(label[idx:], ")"); end >= 0 {
				duration = label[idx+1 : idx+end]
			}
		}
		if totalTests == 0 {
			for _, t := range tables {
				totalTests += len(t.Results)
			}
		}
	}

	// Triage line
	fmt.Fprintf(&sb, "%d %s / %d tests %d pkg", failCount, symError, totalTests, pkgCount)
	if duration != "" {
		sb.WriteString(" (" + duration + ")")
	}
	sb.WriteString("\n")

	// Only render failed tests
	hasFailures := false
	for _, t := range tables {
		for _, item := range t.Results {
			if item.Status != pattern.StatusFail {
				continue
			}
			if !hasFailures {
				sb.WriteString("\n")
				hasFailures = true
			}
			pkg := extractPackage(t.Label)
			sb.WriteString("  " + symError + " " + pkg + " " + item.Name)
			if item.Duration != "" {
				sb.WriteString(" (" + item.Duration + ")")
			}
			sb.WriteString("\n")
			writeDetails(&sb, item.Details)
			writeFixCommand(&sb, item.FixCommand)
		}
	}

	return sb.String()
}

// extractPackage pulls the package name from a test table label.
func extractPackage(label string) string {
	for _, prefix := range []string{"PANIC ", "BUILD FAIL ", "FAIL "} {
		if strings.HasPrefix(label, prefix) {
			rest := label[len(prefix):]
			if idx := strings.Index(rest, " ("); idx >= 0 {
				return rest[:idx]
			}
			return rest
		}
	}
	return label
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

// writeDetails appends a truncated detail block (max maxDetailLines lines, 4-space indent).
func writeDetails(sb *strings.Builder, details string) {
	if details == "" {
		return
	}
	lines := strings.Split(details, "\n")
	show := maxDetailLines
	if len(lines) < show {
		show = len(lines)
	}
	for _, line := range lines[:show] {
		sb.WriteString("    " + line + "\n")
	}
	if len(lines) > maxDetailLines {
		fmt.Fprintf(sb, "    ... (%d more lines)\n", len(lines)-maxDetailLines)
	}
}


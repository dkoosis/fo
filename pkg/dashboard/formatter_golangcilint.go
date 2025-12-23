package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Formatting constants for golangci-lint output.
const (
	lintItemsPerSection  = 5  // max items shown per linter section
	lintComplexityWarn   = 30 // complexity threshold for red highlighting
	lintMsgTruncateLen   = 50 // max message length before truncation
	lintFileListMaxLen   = 40 // max length for file list in goconst
	lintFuncNameColWidth = 24 // column width for function names
)

// GolangciLintFormatter handles golangci-lint output.
type GolangciLintFormatter struct{}

func (f *GolangciLintFormatter) Matches(command string) bool {
	return strings.Contains(command, "golangci-lint")
}

// lintIssue represents a single issue from golangci-lint SARIF output.
type lintIssue struct {
	linter  string
	file    string
	line    int
	message string
	level   string
}

func (f *GolangciLintFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	s := Styles()

	// Parse SARIF - extract JSON from mixed output (stdout SARIF + stderr text)
	var report SARIFReport
	var parsed bool
	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"runs"`) {
			if err := json.Unmarshal([]byte(trimmed), &report); err == nil {
				parsed = true
				break
			}
		}
	}
	if !parsed {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Group issues by linter
	byLinter := make(map[string][]lintIssue)
	totalIssues := 0
	for _, run := range report.Runs {
		for _, result := range run.Results {
			filePath := ""
			lineNum := 0
			if len(result.Locations) > 0 {
				loc := result.Locations[0].PhysicalLocation
				filePath = loc.ArtifactLocation.URI
				lineNum = loc.Region.StartLine
			}
			issue := lintIssue{
				linter:  result.RuleID,
				file:    filePath,
				line:    lineNum,
				message: result.Message.Text,
				level:   result.Level,
			}
			byLinter[result.RuleID] = append(byLinter[result.RuleID], issue)
			totalIssues++
		}
	}

	// No issues
	if len(byLinter) == 0 {
		b.WriteString(s.Success.Render("✓ No issues found\n"))
		return b.String()
	}

	// Sort linters by issue count (descending)
	type linterGroup struct {
		name   string
		issues []lintIssue
	}
	groups := make([]linterGroup, 0, len(byLinter))
	for name, issues := range byLinter {
		groups = append(groups, linterGroup{name, issues})
	}
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].issues) > len(groups[j].issues)
	})

	// Render each linter section
	for _, g := range groups {
		countStyle := s.Warn
		for _, iss := range g.issues {
			if iss.level == statusError {
				countStyle = s.Error
				break
			}
		}

		b.WriteString(s.Header.Render(fmt.Sprintf("◉ %s", g.name)))
		b.WriteString(countStyle.Render(fmt.Sprintf(" (%d)", len(g.issues))))
		b.WriteString("\n")

		// Dispatch to per-linter renderer
		switch g.name {
		case "gocyclo":
			f.renderGocyclo(&b, g.issues, s.File, s.Error, s.Warn, s.Muted)
		case "goconst":
			f.renderGoconst(&b, g.issues, s.File, s.Muted)
		default:
			f.renderDefault(&b, g.issues, s.File, s.Muted, s.Error, s.Warn)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Gocyclo display limits.
const (
	gocycloMaxItems     = 15 // max items to show
	gocycloFileColWidth = 20 // column width for filenames
)

// renderGocyclo renders complexity issues as a ranked list.
func (f *GolangciLintFormatter) renderGocyclo(b *strings.Builder, issues []lintIssue, fileStyle, errorStyle, warnStyle, mutedStyle lipgloss.Style) {
	type complexityItem struct {
		funcName   string
		complexity int
		file       string
	}
	items := make([]complexityItem, 0, len(issues))

	for _, iss := range issues {
		var complexity int
		var funcName string
		// Parse: "cyclomatic complexity 25 of func `run` is high (> 20)"
		if _, err := fmt.Sscanf(iss.message, "cyclomatic complexity %d of func `", &complexity); err == nil {
			if start := strings.Index(iss.message, "`"); start >= 0 {
				if end := strings.Index(iss.message[start+1:], "`"); end >= 0 {
					funcName = iss.message[start+1 : start+1+end]
				}
			}
		}
		if funcName == "" {
			funcName = "?"
		}
		items = append(items, complexityItem{funcName, complexity, iss.file})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].complexity > items[j].complexity
	})

	// Calculate max filename width for alignment
	maxFileLen := 0
	for i, item := range items {
		if i >= gocycloMaxItems {
			break
		}
		if len(shortPath(item.file)) > maxFileLen {
			maxFileLen = len(shortPath(item.file))
		}
	}

	// Format: "54  formatter.go         (*GoTestFormatter).Format"
	for i, item := range items {
		if i >= gocycloMaxItems {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(items)-gocycloMaxItems)))
			break
		}
		scoreStyle := warnStyle
		if item.complexity > lintComplexityWarn {
			scoreStyle = errorStyle
		}
		filename := shortPath(item.file)
		// Pad filename before styling to ensure alignment
		paddedFilename := fmt.Sprintf("%-*s", maxFileLen, filename)
		_, _ = fmt.Fprintf(b, "  %s  %s  %s\n",
			scoreStyle.Render(fmt.Sprintf("%2d", item.complexity)),
			fileStyle.Render(paddedFilename),
			mutedStyle.Render(item.funcName))
	}
}

// Goconst display limits.
const (
	goconstMaxItems      = 15 // max items to show
	goconstLiteralWidth  = 16 // column width for quoted literals
	goconstFileListWidth = 40 // max width for file list
)

// renderGoconst renders magic string issues grouped by literal.
func (f *GolangciLintFormatter) renderGoconst(b *strings.Builder, issues []lintIssue, fileStyle, mutedStyle lipgloss.Style) {
	type constItem struct {
		literal string
		count   int
		files   []string
	}
	byLiteral := make(map[string]*constItem)

	for _, iss := range issues {
		start := strings.Index(iss.message, "`")
		if start < 0 {
			continue
		}
		end := strings.Index(iss.message[start+1:], "`")
		if end < 0 {
			continue
		}
		literal := iss.message[start+1 : start+1+end]

		var count int
		_, _ = fmt.Sscanf(iss.message, "string `"+literal+"` has %d occurrences", &count)

		if byLiteral[literal] == nil {
			byLiteral[literal] = &constItem{literal: literal, count: count}
		}
		// Dedupe files
		filename := shortPath(iss.file)
		found := false
		for _, f := range byLiteral[literal].files {
			if f == filename {
				found = true
				break
			}
		}
		if !found {
			byLiteral[literal].files = append(byLiteral[literal].files, filename)
		}
	}

	items := make([]*constItem, 0, len(byLiteral))
	for _, item := range byLiteral {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	// Calculate max literal width for alignment
	maxLiteralLen := 0
	for i, item := range items {
		if i >= goconstMaxItems {
			break
		}
		quoted := fmt.Sprintf("%q", item.literal)
		if len(quoted) > goconstLiteralWidth {
			quoted = quoted[:goconstLiteralWidth-3] + "...\""
		}
		if len(quoted) > maxLiteralLen {
			maxLiteralLen = len(quoted)
		}
	}

	// Format: " 9x "fail"          formatter.go, housekeeping.go"
	for i, item := range items {
		if i >= goconstMaxItems {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(items)-goconstMaxItems)))
			break
		}
		// Quoted literal, truncate if needed
		quoted := fmt.Sprintf("%q", item.literal)
		if len(quoted) > goconstLiteralWidth {
			quoted = quoted[:goconstLiteralWidth-3] + "...\""
		}
		// Pad before styling
		paddedQuoted := fmt.Sprintf("%-*s", maxLiteralLen, quoted)

		files := strings.Join(item.files, ", ")
		if len(files) > goconstFileListWidth {
			files = files[:goconstFileListWidth-3] + "..."
		}

		_, _ = fmt.Fprintf(b, "  %s %s  %s\n",
			mutedStyle.Render(fmt.Sprintf("%2dx", item.count)),
			mutedStyle.Render(paddedQuoted),
			fileStyle.Render(files))
	}
}

// Default linter display limits.
const (
	defaultMaxItems  = 15 // max items to show
	defaultMsgMaxLen = 70 // max message length before truncation
)

// renderDefault renders issues as a two-line format: file:line then message.
func (f *GolangciLintFormatter) renderDefault(b *strings.Builder, issues []lintIssue, fileStyle, mutedStyle, errorStyle, warnStyle lipgloss.Style) {
	for i, iss := range issues {
		if i >= defaultMaxItems {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(issues)-defaultMaxItems)))
			break
		}
		icon := mutedStyle.Render("·")
		switch iss.level {
		case statusError:
			icon = errorStyle.Render("✗")
		case statusWarning:
			icon = warnStyle.Render("△")
		}
		msg := iss.message
		if len(msg) > defaultMsgMaxLen {
			msg = msg[:defaultMsgMaxLen-3] + "..."
		}
		// Line 1: icon + file:line
		_, _ = fmt.Fprintf(b, "  %s %s\n",
			icon,
			fileStyle.Render(fmt.Sprintf("%s:%d", shortPath(iss.file), iss.line)))
		// Line 2: indented message
		_, _ = fmt.Fprintf(b, "    %s\n", mutedStyle.Render(msg))
	}
}

// shortPath extracts just the filename from a path.
func shortPath(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

package mapper

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/internal/archlint"
	"github.com/dkoosis/fo/internal/fometrics"
	"github.com/dkoosis/fo/internal/jscpd"
	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
)

const (
	statusFail = "fail"
	statusPass = "pass"
)

// FromReport converts multi-section report data into patterns.
// Individual section parse failures are reported as error patterns, not
// as a top-level error — a malformed lint section shouldn't hide passing tests.
func FromReport(sections []report.Section) []pattern.Pattern {
	allPatterns := make([]pattern.Pattern, 0, len(sections)*2)
	toolSummaries := make([]pattern.SummaryItem, 0, len(sections))
	pass, fail := 0, 0

	for _, sec := range sections {
		sectionPatterns, kind, scopeLabel := mapSection(sec)

		if kind == pattern.KindError {
			fail++
		} else {
			pass++
		}

		toolSummaries = append(toolSummaries, pattern.SummaryItem{
			Label: sec.Tool,
			Value: scopeLabel,
			Kind:  kind,
		})

		// Tag tables with their tool source for renderer grouping
		for _, p := range sectionPatterns {
			if t, ok := p.(*pattern.TestTable); ok {
				t.Source = sec.Tool
			}
		}
		allPatterns = append(allPatterns, sectionPatterns...)
	}

	label := fmt.Sprintf("REPORT: %d tools", len(sections))
	if fail == 0 {
		label += " — all pass"
	} else {
		parts := []string{fmt.Sprintf("%d fail", fail)}
		if pass > 0 {
			parts = append(parts, fmt.Sprintf("%d pass", pass))
		}
		label += " — " + strings.Join(parts, ", ")
	}

	topSummary := &pattern.Summary{
		Label:   label,
		Kind:    pattern.SummaryKindReport,
		Metrics: toolSummaries,
	}

	return append([]pattern.Pattern{topSummary}, allPatterns...)
}

func mapSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	switch sec.Format {
	case "sarif":
		return mapSARIFSection(sec)
	case "testjson":
		return mapTestJSONSection(sec)
	case "metrics":
		return mapMetricsSection(sec)
	case "archlint":
		return mapArchLintSection(sec)
	case "jscpd":
		return mapJSCPDSection(sec)
	case "text":
		return mapTextSection(sec)
	default:
		return sectionError(sec.Tool, fmt.Errorf("unknown format %q", sec.Format)),
			pattern.KindError, fmt.Sprintf("unknown format %q", sec.Format)
	}
}

// sectionError emits a visible error pattern for a section that failed to parse.
func sectionError(tool string, err error) []pattern.Pattern {
	return []pattern.Pattern{
		&pattern.Error{Source: tool, Message: err.Error()},
	}
}

func mapSARIFSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	doc, err := sarif.ReadBytes(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}
	stats := sarif.ComputeStats(doc)
	patterns := fromSARIF(doc, stats)

	kind := pattern.KindSuccess
	if stats.ByLevel["error"] > 0 {
		kind = pattern.KindError
	}
	label := fmt.Sprintf("%d diags", stats.TotalIssues)
	if stats.TotalIssues > 0 {
		var parts []string
		if n := stats.ByLevel["error"]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d err", n))
		}
		if n := stats.ByLevel["warning"]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d warn", n))
		}
		if len(parts) > 0 {
			label = strings.Join(parts, ", ")
		}
	}
	return patterns, kind, label
}

func mapTestJSONSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	results, _, err := testjson.ParseBytes(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}
	stats := testjson.ComputeStats(results)
	patterns := fromTestJSON(results, stats)

	passed := stats.Failed == 0 && stats.BuildErrors == 0 && stats.Panics == 0
	if passed {
		label := fmt.Sprintf("PASS — %d tests, %d packages", stats.TotalTests, stats.Packages)
		return patterns, pattern.KindSuccess, label
	}
	label := fmt.Sprintf("FAIL — %d failed, %d passed", stats.Failed, stats.Passed)
	return patterns, pattern.KindError, label
}

func mapMetricsSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	doc, err := fometrics.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}

	kind := mapMetricsStatus(doc.Status)
	label := buildMetricsLabel(doc)

	var patterns []pattern.Pattern

	if len(doc.Details) > 0 {
		items := make([]pattern.TestTableItem, 0, len(doc.Details))
		for _, d := range doc.Details {
			status := mapDetailSeverity(d.Severity)
			name := d.Message
			if d.File != "" {
				loc := d.File
				if d.Line > 0 {
					loc = fmt.Sprintf("%s:%d", d.File, d.Line)
				}
				name = loc + " " + d.Message
			}
			items = append(items, pattern.TestTableItem{
				Name:   name,
				Status: status,
			})
		}
		patterns = append(patterns, &pattern.TestTable{
			Label:   sec.Tool + " details",
			Results: items,
		})
	}

	// Ensure status:"fail" produces exit code 1 even when details are empty.
	if doc.Status == "fail" && len(doc.Details) == 0 {
		patterns = append(patterns, &pattern.TestTable{
			Label: sec.Tool,
			Results: []pattern.TestTableItem{
				{Name: "metrics check failed", Status: "fail"},
			},
		})
	}

	return patterns, kind, label
}

func mapMetricsStatus(status string) pattern.ItemKind {
	switch status {
	case "fail":
		return pattern.KindError
	case "warn":
		return pattern.KindWarning
	default:
		return pattern.KindSuccess
	}
}

func buildMetricsLabel(doc *fometrics.Document) string {
	prefix := strings.ToUpper(doc.Status)

	var parts []string
	for _, m := range doc.Metrics {
		formatted := formatMetricValue(m)
		parts = append(parts, fmt.Sprintf("%s=%s", m.Name, formatted))
	}

	label := prefix
	if len(parts) > 0 {
		label += " — " + strings.Join(parts, " ")
	}
	if doc.Summary != "" {
		label += " (" + doc.Summary + ")"
	}
	return label
}

func formatMetricValue(m fometrics.Metric) string {
	var s string
	if m.Value == float64(int64(m.Value)) {
		s = fmt.Sprintf("%d", int64(m.Value))
	} else {
		s = fmt.Sprintf("%.3f", m.Value)
	}
	if m.Unit != "" {
		s += m.Unit
	}
	return s
}

func mapDetailSeverity(severity string) string {
	switch severity {
	case "error":
		return "fail"
	case "warn":
		return "skip"
	default:
		return "pass"
	}
}

func mapArchLintSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	result, err := archlint.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}

	if !result.HasWarnings {
		label := fmt.Sprintf("pass (%d checks, 0 violations)", len(result.Checks))
		return nil, pattern.KindSuccess, label
	}

	var items []pattern.TestTableItem
	for _, v := range result.Violations {
		items = append(items, pattern.TestTableItem{
			Name:   fmt.Sprintf("%s → %s", v.From, v.To),
			Status: statusFail,
		})
	}
	patterns := []pattern.Pattern{
		&pattern.TestTable{
			Label:   sec.Tool + " violations",
			Results: items,
		},
	}
	label := fmt.Sprintf("FAIL — %d violation", len(result.Violations))
	if len(result.Violations) != 1 {
		label += "s"
	}
	return patterns, pattern.KindError, label
}

func mapJSCPDSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	result, err := jscpd.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}

	if len(result.Clones) == 0 {
		return nil, pattern.KindSuccess, "pass (0 clones)"
	}

	var items []pattern.TestTableItem
	for _, c := range result.Clones {
		items = append(items, pattern.TestTableItem{
			Name:    fmt.Sprintf("%s:%d-%d ↔ %s:%d-%d", c.FileA, c.StartA, c.EndA, c.FileB, c.StartB, c.EndB),
			Status:  "skip",
			Details: fmt.Sprintf("%d lines (%s)", c.Lines, c.Format),
		})
	}
	patterns := []pattern.Pattern{
		&pattern.TestTable{
			Label:   sec.Tool + " clones",
			Results: items,
		},
	}
	label := fmt.Sprintf("%d clones", len(result.Clones))
	return patterns, pattern.KindSuccess, label // clones don't fail the report
}

// mapTextSection handles text sections with explicit pass/fail status.
// Text sections rely on explicit status from the delimiter; content is opaque.
func mapTextSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	kind := pattern.KindSuccess
	if sec.Status == statusFail {
		kind = pattern.KindError
	}
	status := sec.Status
	if status == "" {
		status = statusPass
	}
	label := status
	if len(sec.Content) > 0 {
		firstLine := string(sec.Content)
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		if len(firstLine) > 60 {
			firstLine = firstLine[:60] + "…"
		}
		label = status + " — " + firstLine
	}
	return nil, kind, label
}

package mapper

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/internal/archlint"
	"github.com/dkoosis/fo/internal/jscpd"
	"github.com/dkoosis/fo/internal/metrics"
	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
)

const (
	statusFail = "fail"
	statusPass = "pass"
	kindSuccess = "success"
	kindError   = "error"
)

// FromReport converts multi-section report data into patterns.
// Individual section parse failures are reported as error patterns, not
// as a top-level error — a malformed lint section shouldn't hide passing tests.
func FromReport(sections []report.Section) ([]pattern.Pattern, error) {
	allPatterns := make([]pattern.Pattern, 0, len(sections)*2)
	toolSummaries := make([]pattern.SummaryItem, 0, len(sections))
	pass, fail := 0, 0

	for _, sec := range sections {
		sectionPatterns, sectionPass, scopeLabel := mapSection(sec)

		if sectionPass {
			pass++
		} else {
			fail++
		}

		kind := kindSuccess
		if !sectionPass {
			kind = kindError
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

	return append([]pattern.Pattern{topSummary}, allPatterns...), nil
}

func mapSection(sec report.Section) ([]pattern.Pattern, bool, string) {
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
			false, fmt.Sprintf("unknown format %q", sec.Format)
	}
}

// sectionError emits a visible error pattern for a section that failed to parse.
func sectionError(tool string, err error) []pattern.Pattern {
	return []pattern.Pattern{
		&pattern.Error{Source: tool, Message: err.Error()},
	}
}

func mapSARIFSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	doc, err := sarif.ReadBytes(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), false, fmt.Sprintf("parse error: %v", err)
	}
	stats := sarif.ComputeStats(doc)
	patterns := FromSARIF(doc)

	passed := stats.ByLevel["error"] == 0
	label := fmt.Sprintf("%d diags", stats.TotalIssues)
	if stats.TotalIssues > 0 {
		var parts []string
		if n := stats.ByLevel["error"]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d err", n))
		}
		if n := stats.ByLevel["warning"]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d warn", n))
		}
		label = strings.Join(parts, ", ")
	}
	return patterns, passed, label
}

func mapTestJSONSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	results, err := testjson.ParseBytes(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), false, fmt.Sprintf("parse error: %v", err)
	}
	stats := testjson.ComputeStats(results)
	patterns := FromTestJSON(results)

	passed := stats.Failed == 0 && stats.BuildErrors == 0 && stats.Panics == 0
	if passed {
		label := fmt.Sprintf("PASS — %d tests, %d packages", stats.TotalTests, stats.Packages)
		return patterns, true, label
	}
	label := fmt.Sprintf("FAIL — %d failed, %d passed", stats.Failed, stats.Passed)
	return patterns, false, label
}

func mapMetricsSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	m, err := metrics.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), false, fmt.Sprintf("parse error: %v", err)
	}

	passed := len(m.Regressions) == 0

	var label string
	if len(m.Rows) > 0 && len(m.Columns) > 0 {
		row := m.Rows[0]
		var parts []string
		for i, col := range m.Columns {
			if i < len(row.Values) {
				parts = append(parts, fmt.Sprintf("%s=%.3f", col, row.Values[i]))
			}
		}
		prefix := "PASS"
		if !passed {
			prefix = "FAIL"
		}
		label = fmt.Sprintf("%s — %s (%s", prefix, strings.Join(parts, " "), m.Scope)
		if passed {
			label += ", no regressions)"
		} else {
			label += ")"
		}
	}

	var patterns []pattern.Pattern
	if len(m.Regressions) > 0 {
		var items []pattern.TestTableItem
		for _, r := range m.Regressions {
			items = append(items, pattern.TestTableItem{
				Name:    fmt.Sprintf("regression: %s %s", r.Group, r.Metric),
				Status:  statusFail,
				Details: fmt.Sprintf("%.3f→%.3f (%.3f)", r.From, r.To, r.To-r.From),
			})
		}
		patterns = append(patterns, &pattern.TestTable{
			Label:   sec.Tool + " regressions",
			Results: items,
		})
	}

	return patterns, passed, label
}

func mapArchLintSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	result, err := archlint.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), false, fmt.Sprintf("parse error: %v", err)
	}

	passed := !result.HasWarnings
	if passed {
		label := fmt.Sprintf("pass (%d checks, 0 violations)", len(result.Checks))
		return nil, true, label
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
	return patterns, false, label
}

func mapJSCPDSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	result, err := jscpd.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), false, fmt.Sprintf("parse error: %v", err)
	}

	if len(result.Clones) == 0 {
		return nil, true, "pass (0 clones)"
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
	return patterns, true, label // clones don't fail the report
}

// mapTextSection handles text sections with explicit pass/fail status.
// Text sections rely on explicit status from the delimiter; content is opaque.
func mapTextSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	passed := sec.Status != statusFail
	status := sec.Status
	if status == "" {
		status = "pass"
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
	return nil, passed, label
}

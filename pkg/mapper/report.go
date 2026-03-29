package mapper

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
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
	} else if pass > 0 {
		label += fmt.Sprintf(" — %d fail, %d pass", fail, pass)
	} else {
		label += fmt.Sprintf(" — %d fail", fail)
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
	results, malformed, err := testjson.ParseBytes(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}
	stats := testjson.ComputeStats(results)
	patterns := fromTestJSON(results, stats)

	if malformed > 0 {
		patterns = append(patterns, &pattern.Error{
			Source:  sec.Tool,
			Message: fmt.Sprintf("warning: %d malformed line(s) skipped", malformed),
		})
	}

	passed := stats.Failed == 0 && stats.BuildErrors == 0 && stats.Panics == 0

	var label string
	var kind pattern.ItemKind
	if passed {
		label = fmt.Sprintf("PASS — %d tests, %d packages", stats.TotalTests, stats.Packages)
		kind = pattern.KindSuccess
	} else {
		label = fmt.Sprintf("FAIL — %d failed, %d passed", stats.Failed, stats.Passed)
		kind = pattern.KindError
	}
	if malformed > 0 {
		label += fmt.Sprintf(" (%d malformed lines skipped)", malformed)
	}
	return patterns, kind, label
}


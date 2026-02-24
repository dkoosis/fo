package mapper

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dkoosis/fo/pkg/jtbd"
	"github.com/dkoosis/fo/pkg/pattern"
)

// FromJTBD converts a JTBD report into visualization patterns.
func FromJTBD(report *jtbd.Report) []pattern.Pattern {
	var patterns []pattern.Pattern

	patterns = append(patterns, jtbdSummary(report))

	layers := groupByLayer(report.Jobs)
	for _, layer := range []string{"Plumbing", "Memory", "Timing", "Insight"} {
		jobs := layers[layer]
		if len(jobs) == 0 {
			continue
		}
		patterns = append(patterns, jtbdLayerTable(layer, jobs))
	}

	return patterns
}

func jtbdSummary(r *jtbd.Report) *pattern.Summary {
	var metrics []pattern.SummaryItem

	if r.Running > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Running", Value: fmt.Sprintf("%d", r.Running), Kind: "success",
		})
	}
	if r.Broken > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Broken", Value: fmt.Sprintf("%d", r.Broken), Kind: "error",
		})
	}
	if r.WIP > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Not Yet Built", Value: fmt.Sprintf("%d", r.WIP), Kind: "info",
		})
	}

	label := fmt.Sprintf("JTBD Coverage: %d/%d running", r.Running, r.Total)
	return &pattern.Summary{Label: label, Metrics: metrics}
}

func jtbdLayerTable(layer string, jobs []jtbd.JobResult) *pattern.TestTable {
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].Job.ID < jobs[j].Job.ID
	})

	items := make([]pattern.TestTableItem, 0, len(jobs))
	for _, j := range jobs {
		status := mapJTBDStatus(j.Status)

		item := pattern.TestTableItem{
			Name:   fmt.Sprintf("%-8s %s", j.Job.ID, j.Job.Statement),
			Status: status,
			Count:  j.Total,
		}

		if j.Total > 0 {
			names := make([]string, 0, len(j.Tests))
			for _, t := range j.Tests {
				name := shortJTBDFuncName(t.FuncName)
				if t.Status == "fail" {
					name += " FAIL"
				}
				names = append(names, name)
			}
			item.Details = fmt.Sprintf("%s (%d/%d pass)", strings.Join(names, ", "), j.Passed, j.Total)
		} else {
			item.Details = "(no tests annotated)"
		}

		items = append(items, item)
	}

	return &pattern.TestTable{
		Label:   "Layer: " + layer,
		Results: items,
	}
}

func mapJTBDStatus(s string) string {
	switch s {
	case "running":
		return "pass"
	case "broken":
		return "fail"
	default:
		return "wip"
	}
}

func shortJTBDFuncName(name string) string {
	name = strings.TrimPrefix(name, "Test")
	if len(name) > 30 {
		name = name[:27] + "..."
	}
	return name
}

func groupByLayer(jobs []jtbd.JobResult) map[string][]jtbd.JobResult {
	groups := make(map[string][]jtbd.JobResult)
	for _, j := range jobs {
		groups[j.Job.Layer] = append(groups[j.Job.Layer], j)
	}
	return groups
}

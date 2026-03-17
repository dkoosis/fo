package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/internal/archlint"
	"github.com/dkoosis/fo/internal/fometrics"
)

func runWrapArchlint(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint: reading stdin: %v\n", err)
		return 1
	}

	result, err := archlint.Parse(data)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint: %v\n", err)
		return 1
	}

	doc := fometrics.Document{
		Schema: "fo-metrics/v1",
		Tool:   "go-arch-lint",
		Status: "pass",
		Metrics: []fometrics.Metric{
			{Name: "violations", Value: float64(len(result.Violations)), Direction: "lower_is_better"},
			{Name: "checks", Value: float64(result.CheckCount)},
		},
	}

	if result.HasWarnings {
		doc.Status = "fail"
		for _, v := range result.Violations {
			doc.Details = append(doc.Details, fometrics.Detail{
				Message:  fmt.Sprintf("%s → %s", v.From, v.To),
				File:     v.FileFrom,
				Severity: "error",
				Category: "dependency",
			})
		}
	}

	doc.Summary = fmt.Sprintf("%d violations, %d checks", len(result.Violations), result.CheckCount)

	out, err := json.Marshal(doc)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint: marshal: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", out)
	return 0
}

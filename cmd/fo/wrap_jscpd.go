package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/internal/fometrics"
	"github.com/dkoosis/fo/internal/jscpd"
)

func runWrapJscpd(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap jscpd: reading stdin: %v\n", err)
		return 1
	}

	result, err := jscpd.Parse(data)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap jscpd: %v\n", err)
		return 1
	}

	doc := fometrics.Document{
		Schema: "fo-metrics/v1",
		Tool:   "jscpd",
		Status: "pass",
		Metrics: []fometrics.Metric{
			{Name: "clones", Value: float64(len(result.Clones))},
		},
	}

	if len(result.Clones) > 0 {
		doc.Status = "warn"
		for _, c := range result.Clones {
			doc.Details = append(doc.Details, fometrics.Detail{
				Message:  fmt.Sprintf("%s:%d-%d ↔ %s:%d-%d (%d lines, %s)", c.FileA, c.StartA, c.EndA, c.FileB, c.StartB, c.EndB, c.Lines, c.Format),
				File:     c.FileA,
				Line:     c.StartA,
				Severity: "warn",
			})
		}
	}

	doc.Summary = fmt.Sprintf("%d clones", len(result.Clones))

	out, err := json.Marshal(doc)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap jscpd: marshal: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", out)
	return 0
}

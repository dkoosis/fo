package render_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

type unknownPattern struct{}

func (unknownPattern) Type() pattern.PatternType { return "unknown" }

func TestTerminalRender_ShowsSummaryAndSkipsUnknownPatterns(t *testing.T) {
	r := render.NewTerminal(render.MonoTheme())

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — all pass",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: pattern.KindSuccess},
				{Label: "test", Value: "PASS — 60 tests", Kind: pattern.KindSuccess},
			},
		},
		unknownPattern{},
	})

	for _, want := range []string{"REPORT:", "vet: 0 diags", "test: PASS — 60 tests"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

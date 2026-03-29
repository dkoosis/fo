package mapper_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/mapper"
	"github.com/dkoosis/fo/pkg/pattern"
)

func TestFromReport_EmitsErrorPattern_When_TestJSONSectionIsMalformed(t *testing.T) {
	t.Parallel()

	sections := []report.Section{{
		Tool:    "go-test",
		Format:  "testjson",
		Content: []byte("{" + strings.Repeat("x", (1024*1024)+32)),
	}}

	patterns := mapper.FromReport(sections)
	if len(patterns) < 2 {
		t.Fatalf("expected summary + error pattern, got %d patterns", len(patterns))
	}

	summary, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected first pattern to be *pattern.Summary, got %T", patterns[0])
	}
	if got, want := summary.Metrics[0].Kind, pattern.KindError; got != want {
		t.Fatalf("summary metric kind = %q, want %q", got, want)
	}
	if got := summary.Metrics[0].Value; !strings.Contains(got, "parse error") {
		t.Fatalf("summary metric value should include parse error, got %q", got)
	}

	errPattern, ok := patterns[1].(*pattern.Error)
	if !ok {
		t.Fatalf("expected second pattern to be *pattern.Error, got %T", patterns[1])
	}
	if got, want := errPattern.Source, "go-test"; got != want {
		t.Fatalf("error source = %q, want %q", got, want)
	}
}

func TestFromReport_EmitsErrorPattern_When_SectionFormatIsUnknown(t *testing.T) {
	t.Parallel()

	sections := []report.Section{{
		Tool:    "mystery",
		Format:  "binary",
		Content: []byte("ignored"),
	}}

	patterns := mapper.FromReport(sections)
	summary := mustSummary(t, patterns[0])
	if got, want := summary.Metrics[0].Kind, pattern.KindError; got != want {
		t.Fatalf("summary metric kind = %q, want %q", got, want)
	}
	if got := summary.Metrics[0].Value; !strings.Contains(got, `unknown format "binary"`) {
		t.Fatalf("summary metric should mention unknown format, got %q", got)
	}
}

func TestFromReport_MapsTestJSONAndTagsSource_When_ReportContainsPassingSection(t *testing.T) {
	t.Parallel()

	sections := []report.Section{{
		Tool:   "unit",
		Format: "testjson",
		Content: []byte(strings.Join([]string{
			`{"Action":"run","Package":"example.com/project/pkg/a","Test":"TestA"}`,
			`{"Action":"pass","Package":"example.com/project/pkg/a","Test":"TestA","Elapsed":0.01}`,
			`{"Action":"pass","Package":"example.com/project/pkg/a","Elapsed":0.02}`,
		}, "\n") + "\n"),
	}}

	patterns := mapper.FromReport(sections)
	if len(patterns) < 3 {
		t.Fatalf("expected report summary + test summary + passing table, got %d patterns", len(patterns))
	}

	reportSummary := mustSummary(t, patterns[0])
	if got, want := reportSummary.Metrics[0].Kind, pattern.KindSuccess; got != want {
		t.Fatalf("tool metric kind = %q, want %q", got, want)
	}
	if got, want := reportSummary.Metrics[0].Value, "PASS — 1 tests, 1 packages"; got != want {
		t.Fatalf("tool metric value = %q, want %q", got, want)
	}

	// Invariant: FromReport must tag all emitted test tables with section tool name,
	// so renderers can group by source in multi-tool reports.
	for i, p := range patterns[1:] {
		tbl, ok := p.(*pattern.TestTable)
		if !ok {
			continue
		}
		if got, want := tbl.Source, "unit"; got != want {
			t.Fatalf("test table #%d source = %q, want %q", i+1, got, want)
		}
	}
}

func TestFromReport_ComputesMixedSummaryCounts_When_ReportHasPassAndFailingTools(t *testing.T) {
	t.Parallel()

	sections := []report.Section{
		{
			Tool:   "unit-pass",
			Format: "testjson",
			Content: []byte(strings.Join([]string{
				`{"Action":"run","Package":"example.com/project/pkg/a","Test":"TestA"}`,
				`{"Action":"pass","Package":"example.com/project/pkg/a","Test":"TestA"}`,
				`{"Action":"pass","Package":"example.com/project/pkg/a"}`,
			}, "\n") + "\n"),
		},
		{
			Tool:   "unit-fail",
			Format: "testjson",
			Content: []byte(strings.Join([]string{
				`{"Action":"run","Package":"example.com/project/pkg/b","Test":"TestB"}`,
				`{"Action":"fail","Package":"example.com/project/pkg/b","Test":"TestB"}`,
				`{"Action":"fail","Package":"example.com/project/pkg/b"}`,
			}, "\n") + "\n"),
		},
	}

	patterns := mapper.FromReport(sections)
	reportSummary := mustSummary(t, patterns[0])
	if got := reportSummary.Label; !strings.Contains(got, "1 fail, 1 pass") {
		t.Fatalf("report label should include mixed counts, got %q", got)
	}
	if got, want := len(reportSummary.Metrics), 2; got != want {
		t.Fatalf("metric count = %d, want %d", got, want)
	}
}

func mustSummary(t *testing.T, p pattern.Pattern) *pattern.Summary {
	t.Helper()
	summary, ok := p.(*pattern.Summary)
	if !ok {
		t.Fatalf("expected *pattern.Summary, got %T", p)
	}
	return summary
}

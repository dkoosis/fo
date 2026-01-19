package archlint_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/pkg/archlint"
	"github.com/dkoosis/fo/pkg/design"
)

func TestMapper_ReturnsExpectedPatterns_When_ResultStatesVary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  archlint.Result
		inspect func(t *testing.T, patterns []design.Pattern)
	}{
		{
			name: "violations: leaderboard and tables are present",
			result: archlint.Result{
				Payload: archlint.Payload{
					ModuleName: "github.com/example/project",
					ArchWarningsDeps: []archlint.DepWarning{
						{
							ComponentFrom: "domain",
							ComponentTo:   "infra",
							FileFrom:      "internal/domain/service.go",
							Reference:     archlint.Reference{Line: 12, Name: "db.Connect"},
						},
						{
							ComponentFrom: "domain",
							ComponentTo:   "infra",
							FileFrom:      "internal/domain/handler.go",
							Reference:     archlint.Reference{Line: 20, Name: "cache.Get"},
						},
						{
							ComponentFrom: "app",
							ComponentTo:   "infra",
							FileFrom:      "internal/app/main.go",
							Reference:     archlint.Reference{Line: 7, Name: "db.Connect"},
						},
					},
					ArchWarningsNotMatched: []string{"scripts/helper.go"},
					ArchWarningsDeepScan: []archlint.DeepScanWarn{
						{
							ComponentFrom: "domain",
							ComponentTo:   "infra",
							FileFrom:      "internal/domain/query.go",
							Reference:     archlint.Reference{Line: 44, Name: "service.Call"},
						},
					},
					OmittedCount: 2,
				},
			},
			inspect: func(t *testing.T, patterns []design.Pattern) {
				require.Len(t, patterns, 5)

				summary, ok := patterns[0].(*design.Summary)
				require.True(t, ok, "summary pattern should be first")

				wantSummary := &design.Summary{
					Label: "Architecture Check: 5 violations",
					Metrics: []design.SummaryItem{
						{Label: "Status", Value: "FAIL", Type: "error"},
						{Label: "Import Violations", Value: "3", Type: "error"},
						{Label: "Unmatched Files", Value: "1", Type: "warning"},
						{Label: "Deep Scan Warnings", Value: "1", Type: "warning"},
						{Label: "Omitted", Value: "2", Type: "muted"},
					},
				}
				if diff := cmp.Diff(wantSummary, summary); diff != "" {
					t.Fatalf("summary mismatch (-want +got):\n%s", diff)
				}

				leaderboard, ok := patterns[1].(*design.Leaderboard)
				require.True(t, ok, "leaderboard should be second")

				wantLeaderboard := &design.Leaderboard{
					Label:      "Components with Most Violations",
					MetricName: "Violations",
					Items: []design.LeaderboardItem{
						{Name: "domain", Metric: "3 violations", Value: 3, Rank: 1},
						{Name: "app", Metric: "1 violations", Value: 1, Rank: 2},
					},
					Direction:  "highest",
					TotalCount: 2,
					ShowRank:   true,
				}
				if diff := cmp.Diff(wantLeaderboard, leaderboard); diff != "" {
					t.Fatalf("leaderboard mismatch (-want +got):\n%s", diff)
				}

				for _, item := range leaderboard.Items {
					assert.Greater(t, item.Value, 0.0, "ranked components must have violations")
					assert.NotEmpty(t, item.Name, "leaderboard item should include component name")
				}

				appTable, ok := patterns[2].(*design.TestTable)
				require.True(t, ok, "app table should be third")

				wantAppTable := &design.TestTable{
					Label:   "app (1 violations)",
					Results: []design.TestTableItem{{Name: "main.go:7", Status: "fail", Details: "imports infra (db.Connect)"}},
					Density: design.DensityDetailed,
				}
				if diff := cmp.Diff(wantAppTable, appTable); diff != "" {
					t.Fatalf("app table mismatch (-want +got):\n%s", diff)
				}

				domainTable, ok := patterns[3].(*design.TestTable)
				require.True(t, ok, "domain table should be fourth")

				wantDomainTable := &design.TestTable{
					Label: "domain (2 violations)",
					Results: []design.TestTableItem{
						{Name: "service.go:12", Status: "fail", Details: "imports infra (db.Connect)"},
						{Name: "handler.go:20", Status: "fail", Details: "imports infra (cache.Get)"},
					},
					Density: design.DensityDetailed,
				}
				if diff := cmp.Diff(wantDomainTable, domainTable); diff != "" {
					t.Fatalf("domain table mismatch (-want +got):\n%s", diff)
				}

				unmatchedTable, ok := patterns[4].(*design.TestTable)
				require.True(t, ok, "unmatched table should be last")

				wantUnmatchedTable := &design.TestTable{
					Label:   "Unmatched Files (1)",
					Results: []design.TestTableItem{{Name: "helper.go", Status: "skip", Details: "not matched to any component"}},
					Density: design.DensityCompact,
				}
				if diff := cmp.Diff(wantUnmatchedTable, unmatchedTable); diff != "" {
					t.Fatalf("unmatched table mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "clean: summary reports OK",
			result: archlint.Result{
				Payload: archlint.Payload{
					ModuleName: "github.com/example/project",
				},
			},
			inspect: func(t *testing.T, patterns []design.Pattern) {
				require.Len(t, patterns, 1)

				summary, ok := patterns[0].(*design.Summary)
				require.True(t, ok, "summary pattern should be present")

				wantSummary := &design.Summary{
					Label: "Architecture Check",
					Metrics: []design.SummaryItem{
						{Label: "Status", Value: "OK", Type: "success"},
						{Label: "Module", Value: filepath.Base("github.com/example/project"), Type: "info"},
					},
				}
				if diff := cmp.Diff(wantSummary, summary); diff != "" {
					t.Fatalf("summary mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mapper := archlint.NewMapper()
			patterns := mapper.MapToPatterns(&tt.result)

			if tt.inspect == nil {
				require.Fail(t, fmt.Sprintf("missing inspection for %s", tt.name))
			}
			tt.inspect(t, patterns)
		})
	}
}

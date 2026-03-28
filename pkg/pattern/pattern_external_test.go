package pattern_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
)

func TestPatternTypes_AreStable_When_ConstructingKnownPatternValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item pattern.Pattern
		want pattern.PatternType
	}{
		{name: "summary", item: &pattern.Summary{}, want: pattern.PatternTypeSummary},
		{name: "leaderboard", item: &pattern.Leaderboard{}, want: pattern.PatternTypeLeaderboard},
		{name: "test table", item: &pattern.TestTable{}, want: pattern.PatternTypeTestTable},
		{name: "sparkline", item: &pattern.Sparkline{}, want: pattern.PatternTypeSparkline},
		{name: "comparison", item: &pattern.Comparison{}, want: pattern.PatternTypeComparison},
		{name: "error", item: &pattern.Error{}, want: pattern.PatternTypeError},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.item.Type(); got != tt.want {
				t.Fatalf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummaryKinds_AreDistinct_When_UsedForRenderDispatch(t *testing.T) {
	t.Parallel()

	kinds := []pattern.SummaryKind{pattern.SummaryKindSARIF, pattern.SummaryKindTest, pattern.SummaryKindReport}
	seen := make(map[pattern.SummaryKind]struct{}, len(kinds))
	for _, kind := range kinds {
		if _, ok := seen[kind]; ok {
			t.Fatalf("duplicate SummaryKind detected: %q", kind)
		}
		seen[kind] = struct{}{}
	}
}

func TestTestTableStatuses_AreDistinct_When_FilteringRowsByOutcome(t *testing.T) {
	t.Parallel()

	statuses := []pattern.Status{pattern.StatusPass, pattern.StatusFail, pattern.StatusSkip}
	seen := make(map[pattern.Status]struct{}, len(statuses))
	for _, status := range statuses {
		if _, ok := seen[status]; ok {
			t.Fatalf("duplicate Status detected: %q", status)
		}
		seen[status] = struct{}{}
	}
}

func TestFormats_AreNonEmpty_When_InteractingAcrossPackages(t *testing.T) {
	t.Parallel()

	items := []struct {
		name string
		kind string
	}{
		{name: "summary kind sarif", kind: string(pattern.SummaryKindSARIF)},
		{name: "summary kind test", kind: string(pattern.SummaryKindTest)},
		{name: "summary kind report", kind: string(pattern.SummaryKindReport)},
		{name: "item success", kind: string(pattern.KindSuccess)},
		{name: "item error", kind: string(pattern.KindError)},
		{name: "item warning", kind: string(pattern.KindWarning)},
		{name: "item info", kind: string(pattern.KindInfo)},
		{name: "status pass", kind: string(pattern.StatusPass)},
		{name: "status fail", kind: string(pattern.StatusFail)},
		{name: "status skip", kind: string(pattern.StatusSkip)},
	}

	for _, tt := range items {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.kind == "" {
				t.Fatalf("%s must be non-empty", tt.name)
			}
		})
	}
}

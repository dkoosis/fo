package pattern_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
)

func TestPatternType_ReturnsExpectedContractValue_WhenUsingKnownPatternConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  pattern.PatternType
		want string
	}{
		{name: "summary", got: pattern.PatternTypeSummary, want: "summary"},
		{name: "leaderboard", got: pattern.PatternTypeLeaderboard, want: "leaderboard"},
		{name: "test-table", got: pattern.PatternTypeTestTable, want: "test-table"},
		{name: "error", got: pattern.PatternTypeError, want: "error"},
	}

	seen := map[pattern.PatternType]bool{}
	for _, tt := range tests {
		if seen[tt.got] {
			t.Fatalf("PatternType %q duplicated in contract table", tt.got)
		}
		seen[tt.got] = true
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.got) != tt.want {
				t.Fatalf("PatternType constant mismatch: got %q want %q", tt.got, tt.want)
			}
			if tt.got == "" {
				t.Fatal("PatternType should never be empty")
			}
		})
	}
}

func TestPatternType_ReturnsExpectedType_WhenPatternImplementsInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   pattern.Pattern
		want pattern.PatternType
	}{
		{name: "error pointer", in: &pattern.Error{}, want: pattern.PatternTypeError},
		{name: "summary pointer", in: &pattern.Summary{}, want: pattern.PatternTypeSummary},
		{name: "leaderboard pointer", in: &pattern.Leaderboard{}, want: pattern.PatternTypeLeaderboard},
		{name: "test table pointer", in: &pattern.TestTable{}, want: pattern.PatternTypeTestTable},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.in == nil {
				t.Fatal("test setup error: pattern must not be nil")
			}

			got := tt.in.Type()
			if got != tt.want {
				t.Fatalf("Type() mismatch: got %q want %q", got, tt.want)
			}
			if got == "" {
				t.Fatal("Type() should never return empty pattern type")
			}
		})
	}
}

func TestPatternType_DoesNotPanic_WhenTypeCalledOnNilReceiver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   pattern.Pattern
		want pattern.PatternType
	}{
		{name: "nil error", in: (*pattern.Error)(nil), want: pattern.PatternTypeError},
		{name: "nil summary", in: (*pattern.Summary)(nil), want: pattern.PatternTypeSummary},
		{name: "nil leaderboard", in: (*pattern.Leaderboard)(nil), want: pattern.PatternTypeLeaderboard},
		{name: "nil test table", in: (*pattern.TestTable)(nil), want: pattern.PatternTypeTestTable},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Type(); got != tt.want {
				t.Fatalf("nil receiver Type() mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestSummaryKind_ReturnsExpectedContractValue_WhenUsingKnownSummaryKindConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  pattern.SummaryKind
		want string
	}{
		{name: "sarif", got: pattern.SummaryKindSARIF, want: "sarif"},
		{name: "test", got: pattern.SummaryKindTest, want: "test"},
		{name: "report", got: pattern.SummaryKindReport, want: "report"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.got) != tt.want {
				t.Fatalf("SummaryKind constant mismatch: got %q want %q", tt.got, tt.want)
			}
			if tt.got == "" {
				t.Fatal("SummaryKind should never be empty")
			}
		})
	}
}

func TestItemKind_ReturnsExpectedContractValue_WhenUsingKnownItemKindConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  pattern.ItemKind
		want string
	}{
		{name: "success", got: pattern.KindSuccess, want: "success"},
		{name: "error", got: pattern.KindError, want: "error"},
		{name: "warning", got: pattern.KindWarning, want: "warning"},
		{name: "info", got: pattern.KindInfo, want: "info"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.got) != tt.want {
				t.Fatalf("ItemKind constant mismatch: got %q want %q", tt.got, tt.want)
			}
			if tt.got == "" {
				t.Fatal("ItemKind should never be empty")
			}
		})
	}
}

func TestStatus_ReturnsExpectedContractValue_WhenUsingKnownStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  pattern.Status
		want string
	}{
		{name: "pass", got: pattern.StatusPass, want: "pass"},
		{name: "fail", got: pattern.StatusFail, want: "fail"},
		{name: "skip", got: pattern.StatusSkip, want: "skip"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.got) != tt.want {
				t.Fatalf("Status constant mismatch: got %q want %q", tt.got, tt.want)
			}
			if tt.got == "" {
				t.Fatal("Status should never be empty")
			}
		})
	}
}

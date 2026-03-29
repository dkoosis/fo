package render_test

import (
	"encoding/json"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

func TestJSONRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []pattern.Pattern
		check    func(t *testing.T, raw string)
	}{
		{
			name:     "empty patterns produces valid JSON with empty array",
			patterns: nil,
			check: func(t *testing.T, raw string) {
				var out jsonEnvelope
				mustUnmarshal(t, raw, &out)
				assertEqual(t, out.Version, "2.0")
				assertEqual(t, len(out.Patterns), 0)
			},
		},
		{
			name: "summary pattern round-trips with correct type",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label: "test report",
					Kind:  pattern.SummaryKindReport,
					Metrics: []pattern.SummaryItem{
						{Label: "vet", Value: "0 diags", Kind: pattern.KindSuccess},
					},
				},
			},
			check: func(t *testing.T, raw string) {
				var out jsonEnvelope
				mustUnmarshal(t, raw, &out)
				assertEqual(t, len(out.Patterns), 1)
				assertEqual(t, out.Patterns[0].Type, "summary")
			},
		},
		{
			name: "leaderboard pattern has correct type",
			patterns: []pattern.Pattern{
				&pattern.Leaderboard{
					Label: "top files",
					Items: []pattern.LeaderboardItem{
						{Name: "foo.go", Metric: "12", Rank: 1},
					},
				},
			},
			check: func(t *testing.T, raw string) {
				var out jsonEnvelope
				mustUnmarshal(t, raw, &out)
				assertEqual(t, out.Patterns[0].Type, "leaderboard")
			},
		},
		{
			name: "test_table pattern has correct type",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label:   "failures",
					Results: []pattern.TestTableItem{{Name: "TestFoo", Status: pattern.StatusFail}},
				},
			},
			check: func(t *testing.T, raw string) {
				var out jsonEnvelope
				mustUnmarshal(t, raw, &out)
				assertEqual(t, out.Patterns[0].Type, "test-table")
			},
		},
		{
			name: "error pattern has correct type",
			patterns: []pattern.Pattern{
				&pattern.Error{Source: "lint", Message: "crashed"},
			},
			check: func(t *testing.T, raw string) {
				var out jsonEnvelope
				mustUnmarshal(t, raw, &out)
				assertEqual(t, out.Patterns[0].Type, "error")
			},
		},
		{
			name: "multiple patterns preserve order",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "first", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{Label: "second"},
				&pattern.Error{Source: "third", Message: "boom"},
			},
			check: func(t *testing.T, raw string) {
				var out jsonEnvelope
				mustUnmarshal(t, raw, &out)
				assertEqual(t, len(out.Patterns), 3)
				assertEqual(t, out.Patterns[0].Type, "summary")
				assertEqual(t, out.Patterns[1].Type, "test-table")
				assertEqual(t, out.Patterns[2].Type, "error")
			},
		},
		{
			name:     "output ends with newline",
			patterns: []pattern.Pattern{&pattern.Summary{Label: "x", Kind: pattern.SummaryKindTest}},
			check: func(t *testing.T, raw string) {
				if raw[len(raw)-1] != '\n' {
					t.Fatal("expected trailing newline")
				}
			},
		},
	}

	r := render.NewJSON()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := r.Render(tc.patterns)
			tc.check(t, out)
		})
	}
}

// jsonEnvelope mirrors the JSON output structure for test assertions.
type jsonEnvelope struct {
	Version  string `json:"version"`
	Patterns []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"patterns"`
}

func mustUnmarshal(t *testing.T, raw string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, raw)
	}
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

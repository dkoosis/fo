package render_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

type unsupportedJSONPattern struct {
	C chan int
}

func (unsupportedJSONPattern) Type() pattern.PatternType { return "unsupported" }

type renderedJSON struct {
	Version  string `json:"version"`
	Patterns []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"patterns"`
}

func TestJSONRender_ReturnsErrorEnvelope_When_PatternContainsUnsupportedJSONValue(t *testing.T) {
	t.Parallel()

	r := render.NewJSON()
	out := r.Render([]pattern.Pattern{unsupportedJSONPattern{C: make(chan int)}})

	if !json.Valid([]byte(out)) {
		t.Fatalf("expected a valid JSON error envelope, got %q", out)
	}
	if !strings.Contains(out, "unsupported type") {
		t.Fatalf("expected marshal error details in output, got %q", out)
	}
	if strings.HasSuffix(out, "\n") {
		t.Fatalf("expected error envelope to avoid trailing newline, got %q", out)
	}
}

func TestJSONRender_EncodesPatternMetadata_When_PatternsAreProvided(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []pattern.Pattern
		wantType string
		wantKey  string
		wantVal  string
	}{
		{
			name: "summary pattern preserves type and key fields",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label: "report",
					Kind:  pattern.SummaryKindReport,
					Metrics: []pattern.SummaryItem{
						{Label: "test", Value: "PASS", Kind: pattern.KindSuccess},
					},
				},
			},
			wantType: string(pattern.PatternTypeSummary),
			wantKey:  "Label",
			wantVal:  "report",
		},
		{
			name: "error pattern preserves source for downstream automation",
			patterns: []pattern.Pattern{
				&pattern.Error{Source: "sarif", Message: "decode failed"},
			},
			wantType: string(pattern.PatternTypeError),
			wantKey:  "Source",
			wantVal:  "sarif",
		},
	}

	r := render.NewJSON()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out := r.Render(tc.patterns)
			if !strings.HasSuffix(out, "\n") {
				t.Fatalf("expected newline-terminated output, got %q", out)
			}

			var got renderedJSON
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("failed to parse renderer output: %v\noutput: %s", err, out)
			}

			if got.Version != "2.0" {
				t.Fatalf("expected version 2.0, got %q", got.Version)
			}
			if len(got.Patterns) != len(tc.patterns) {
				t.Fatalf("expected %d patterns, got %d", len(tc.patterns), len(got.Patterns))
			}
			if got.Patterns[0].Type != tc.wantType {
				t.Fatalf("expected type %q, got %q", tc.wantType, got.Patterns[0].Type)
			}
			var data map[string]any
			if err := json.Unmarshal(got.Patterns[0].Data, &data); err != nil {
				t.Fatalf("failed to parse pattern payload: %v", err)
			}
			if data[tc.wantKey] != tc.wantVal {
				t.Fatalf("expected %s=%q, got %#v", tc.wantKey, tc.wantVal, data[tc.wantKey])
			}
		})
	}
}

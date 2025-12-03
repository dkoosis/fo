package adapter

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestHousekeepingAdapter_Name(t *testing.T) {
	a := &HousekeepingAdapter{}
	if got := a.Name(); got != "housekeeping" {
		t.Errorf("Name() = %q, want %q", got, "housekeeping")
	}
}

func TestHousekeepingAdapter_Detect(t *testing.T) {
	a := &HousekeepingAdapter{}

	tests := []struct {
		name       string
		firstLines []string
		want       bool
	}{
		{
			name:       "empty input",
			firstLines: []string{},
			want:       false,
		},
		{
			name: "valid housekeeping JSON",
			firstLines: []string{
				`{"checks": [{"name": "markdown_count", "status": "warn", "current": 62, "threshold": 50}]}`,
			},
			want: true,
		},
		{
			name: "valid with multiple checks",
			firstLines: []string{
				`{"checks": [{"name": "todo_comments", "status": "pass"}, {"name": "orphan_tests", "status": "warn"}]}`,
			},
			want: true,
		},
		{
			name: "pretty printed JSON",
			firstLines: []string{
				`{`,
				`  "checks": [`,
				`    {"name": "markdown_count", "status": "warn", "current": 62, "threshold": 50}`,
				`  ]`,
				`}`,
			},
			want: true,
		},
		{
			name: "go test JSON - should not match",
			firstLines: []string{
				`{"Action":"run","Package":"pkg/example","Test":"TestFoo"}`,
			},
			want: false,
		},
		{
			name: "complexity snapshot - should not match",
			firstLines: []string{
				`{"metrics": {"files_over_500": 49}, "hotspots": []}`,
			},
			want: false,
		},
		{
			name: "mcp interviewer - should not match",
			firstLines: []string{
				`{"initialize_result": {"protocolVersion": "1.0"}, "tools": []}`,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.Detect(tt.firstLines)
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHousekeepingAdapter_Parse(t *testing.T) {
	a := &HousekeepingAdapter{}

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, p design.Pattern)
	}{
		{
			name: "full housekeeping with all fields",
			input: `{
				"title": "HOUSEKEEPING",
				"checks": [
					{
						"name": "markdown_count",
						"status": "warn",
						"current": 62,
						"threshold": 50,
						"details": "",
						"items": []
					},
					{
						"name": "todo_comments",
						"status": "warn",
						"current": 23,
						"threshold": 0,
						"details": "7 older than 90 days",
						"items": ["pkg/server/handler.go:42", "pkg/client/api.go:156"]
					},
					{
						"name": "orphan_tests",
						"status": "pass",
						"current": 0,
						"threshold": 0
					}
				]
			}`,
			wantErr: false,
			check: func(t *testing.T, p design.Pattern) {
				h, ok := p.(*design.Housekeeping)
				if !ok {
					t.Fatal("expected Housekeeping")
				}

				if h.Title != "HOUSEKEEPING" {
					t.Errorf("Title = %q, want %q", h.Title, "HOUSEKEEPING")
				}

				if len(h.Checks) != 3 {
					t.Errorf("expected 3 checks, got %d", len(h.Checks))
				}

				// Check first check (markdown_count)
				c := h.Checks[0]
				if c.Name != "Markdown files" {
					t.Errorf("check name = %q, want %q", c.Name, "Markdown files")
				}
				if c.Status != "warn" {
					t.Errorf("check status = %q, want %q", c.Status, "warn")
				}
				if c.Current != 62 {
					t.Errorf("check current = %d, want 62", c.Current)
				}
				if c.Threshold != 50 {
					t.Errorf("check threshold = %d, want 50", c.Threshold)
				}

				// Check second check (todo_comments)
				c = h.Checks[1]
				if c.Name != "TODO comments" {
					t.Errorf("check name = %q, want %q", c.Name, "TODO comments")
				}
				if c.Details != "7 older than 90 days" {
					t.Errorf("check details = %q, want %q", c.Details, "7 older than 90 days")
				}
				if len(c.Items) != 2 {
					t.Errorf("expected 2 items, got %d", len(c.Items))
				}

				// Check third check (orphan_tests)
				c = h.Checks[2]
				if c.Name != "Orphan test files" {
					t.Errorf("check name = %q, want %q", c.Name, "Orphan test files")
				}
				if c.Status != "pass" {
					t.Errorf("check status = %q, want %q", c.Status, "pass")
				}
			},
		},
		{
			name: "minimal input",
			input: `{
				"checks": [
					{"name": "dead_code", "current": 5}
				]
			}`,
			wantErr: false,
			check: func(t *testing.T, p design.Pattern) {
				h, ok := p.(*design.Housekeeping)
				if !ok {
					t.Fatal("expected Housekeeping")
				}

				// Should default title
				if h.Title != "HOUSEKEEPING" {
					t.Errorf("Title = %q, want default %q", h.Title, "HOUSEKEEPING")
				}

				// Should default status to pass
				if h.Checks[0].Status != "pass" {
					t.Errorf("status = %q, want default %q", h.Checks[0].Status, "pass")
				}

				// Check name should be formatted
				if h.Checks[0].Name != "Dead code" {
					t.Errorf("name = %q, want %q", h.Checks[0].Name, "Dead code")
				}
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json`,
			wantErr: true,
		},
		{
			name:    "empty checks array",
			input:   `{"checks": []}`,
			wantErr: false,
			check: func(t *testing.T, p design.Pattern) {
				h, ok := p.(*design.Housekeeping)
				if !ok {
					t.Fatal("expected Housekeeping")
				}
				if len(h.Checks) != 0 {
					t.Errorf("expected 0 checks, got %d", len(h.Checks))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			pattern, err := a.Parse(reader)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pattern == nil {
				t.Fatal("expected pattern, got nil")
			}

			if tt.check != nil {
				tt.check(t, pattern)
			}
		})
	}
}

func TestFormatCheckName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"markdown_count", "Markdown files"},
		{"todo_comments", "TODO comments"},
		{"orphan_tests", "Orphan test files"},
		{"package_docs", "Package documentation"},
		{"dead_code", "Dead code"},
		{"deprecated_deps", "Deprecated dependencies"},
		{"license_headers", "License headers"},
		{"generated_freshness", "Generated file freshness"},
		{"unknown_check", "Unknown Check"}, // fallback to Title Case
		{"single", "Single"},
		{"multi_word_check", "Multi Word Check"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatCheckName(tt.input)
			if got != tt.want {
				t.Errorf("formatCheckName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

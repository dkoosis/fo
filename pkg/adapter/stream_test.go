package adapter

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestGoTestJSONAdapter_Detect(t *testing.T) {
	adapter := &GoTestJSONAdapter{}

	tests := []struct {
		name       string
		firstLines []string
		want       bool
	}{
		{
			name: "valid go test json",
			firstLines: []string{
				`{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example","Test":"TestFoo"}`,
			},
			want: true,
		},
		{
			name: "valid go test json with multiple lines",
			firstLines: []string{
				`{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}`,
				`{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Elapsed":0.1}`,
			},
			want: true,
		},
		{
			name: "empty lines then json",
			firstLines: []string{
				"",
				`{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}`,
			},
			want: true,
		},
		{
			name:       "empty input",
			firstLines: []string{},
			want:       false,
		},
		{
			name: "plain text output",
			firstLines: []string{
				"=== RUN   TestFoo",
				"--- PASS: TestFoo (0.00s)",
			},
			want: false,
		},
		{
			name: "json but not go test format",
			firstLines: []string{
				`{"foo":"bar","baz":123}`,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.Detect(tt.firstLines)
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoTestJSONAdapter_Parse(t *testing.T) {
	adapter := &GoTestJSONAdapter{}

	tests := []struct {
		name         string
		input        string
		wantPackages int
		wantLabel    string
		wantStatus   map[string]string // package name -> status
	}{
		{
			name: "single package pass",
			input: `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}
{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example","Test":"TestFoo"}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Test":"TestFoo","Elapsed":0.1}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Elapsed":0.1}`,
			wantPackages: 1,
			wantLabel:    "Go Test Results",
			wantStatus: map[string]string{
				"pkg/example": "pass",
			},
		},
		{
			name: "single package fail",
			input: `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}
{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example","Test":"TestFoo"}
{"Time":"2024-01-01T12:00:01Z","Action":"fail","Package":"pkg/example","Test":"TestFoo","Elapsed":0.1}
{"Time":"2024-01-01T12:00:01Z","Action":"fail","Package":"pkg/example","Elapsed":0.1}`,
			wantPackages: 1,
			wantLabel:    "Go Test Results",
			wantStatus: map[string]string{
				"pkg/example": "fail",
			},
		},
		{
			name: "multiple packages mixed results",
			input: `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/api"}
{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/api","Test":"TestAPI"}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/api","Test":"TestAPI","Elapsed":0.1}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/api","Elapsed":0.1}
{"Time":"2024-01-01T12:00:01Z","Action":"run","Package":"pkg/db"}
{"Time":"2024-01-01T12:00:01Z","Action":"run","Package":"pkg/db","Test":"TestDB"}
{"Time":"2024-01-01T12:00:02Z","Action":"fail","Package":"pkg/db","Test":"TestDB","Elapsed":0.2}
{"Time":"2024-01-01T12:00:02Z","Action":"fail","Package":"pkg/db","Elapsed":0.2}`,
			wantPackages: 2,
			wantLabel:    "Go Test Results",
			wantStatus: map[string]string{
				"pkg/api": "pass",
				"pkg/db":  "fail",
			},
		},
		{
			name: "skipped tests",
			input: `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}
{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example","Test":"TestFoo"}
{"Time":"2024-01-01T12:00:01Z","Action":"skip","Package":"pkg/example","Test":"TestFoo","Elapsed":0.0}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Elapsed":0.0}`,
			wantPackages: 1,
			wantLabel:    "Go Test Results",
			wantStatus: map[string]string{
				"pkg/example": "skip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			pattern, err := adapter.Parse(reader)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			table, ok := pattern.(*design.TestTable)
			if !ok {
				t.Fatalf("Parse() returned wrong type: %T", pattern)
			}

			if table.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", table.Label, tt.wantLabel)
			}

			if len(table.Results) != tt.wantPackages {
				t.Errorf("got %d packages, want %d", len(table.Results), tt.wantPackages)
			}

			// Check package statuses
			for _, result := range table.Results {
				wantStatus, exists := tt.wantStatus[result.Name]
				if !exists {
					t.Errorf("unexpected package: %s", result.Name)
					continue
				}
				if result.Status != wantStatus {
					t.Errorf("package %s: status = %q, want %q", result.Name, result.Status, wantStatus)
				}
			}
		})
	}
}

func TestRegistry_Detect(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name       string
		firstLines []string
		wantName   string // expected adapter name, or "" for no match
	}{
		{
			name: "go test json detected",
			firstLines: []string{
				`{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}`,
			},
			wantName: "go-test-json",
		},
		{
			name: "plain text not detected",
			firstLines: []string{
				"=== RUN   TestFoo",
			},
			wantName: "",
		},
		{
			name:       "empty input not detected",
			firstLines: []string{},
			wantName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := registry.Detect(tt.firstLines)
			if tt.wantName == "" {
				if adapter != nil {
					t.Errorf("Detect() returned adapter %q, want nil", adapter.Name())
				}
			} else {
				if adapter == nil {
					t.Fatalf("Detect() returned nil, want adapter %q", tt.wantName)
				}
				if adapter.Name() != tt.wantName {
					t.Errorf("Detect() returned adapter %q, want %q", adapter.Name(), tt.wantName)
				}
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0.0001, "0.0s"},
		{0.5, "500ms"},
		{0.999, "999ms"},
		{1.0, "1.0s"},
		{1.234, "1.2s"},
		{12.56, "12.6s"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.seconds)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

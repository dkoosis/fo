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

func TestMCPInterviewerAdapter_Name(t *testing.T) {
	adapter := &MCPInterviewerAdapter{}
	if got := adapter.Name(); got != "mcp-interviewer" {
		t.Errorf("Name() = %q, want %q", got, "mcp-interviewer")
	}
}

func TestMCPInterviewerAdapter_Detect(t *testing.T) {
	adapter := &MCPInterviewerAdapter{}

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
			name: "valid MCP interviewer with initialize_result and protocolVersion",
			firstLines: []string{
				`{"initialize_result":{"protocolVersion":"2024-11-05","serverInfo":{"name":"orca"}},"tools":[]}`,
			},
			want: true,
		},
		{
			name: "valid with tools and serverInfo",
			firstLines: []string{
				`{"serverInfo":{"name":"test"},"tools":[{"name":"foo"}]}`,
			},
			want: true,
		},
		{
			name: "valid with tool_scorecards and initialize_result",
			firstLines: []string{
				`{"initialize_result":{},"tool_scorecards":[]}`,
			},
			want: true,
		},
		{
			name: "pretty printed JSON across multiple lines",
			firstLines: []string{
				`{`,
				`  "initialize_result": {`,
				`    "protocolVersion": "2024-11-05"`,
				`  }`,
				`}`,
			},
			want: true,
		},
		{
			name: "go test JSON - should not match",
			firstLines: []string{
				`{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}`,
			},
			want: false,
		},
		{
			name: "complexity snapshot - should not match",
			firstLines: []string{
				`{"metrics":{"files_over_500":10},"hotspots":[]}`,
			},
			want: false,
		},
		{
			name: "housekeeping JSON - should not match",
			firstLines: []string{
				`{"checks":[{"name":"markdown_count","status":"warn"}]}`,
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

func TestMCPInterviewerAdapter_Parse(t *testing.T) {
	adapter := &MCPInterviewerAdapter{}

	tests := []struct {
		name          string
		input         string
		wantErr       bool
		wantServer    string
		wantToolCount int
		wantCategs    int
	}{
		{
			name: "full MCP interviewer output",
			input: `{
				"initialize_result": {
					"protocolVersion": "2024-11-05",
					"serverInfo": {"name": "orca", "version": "1.0.0"}
				},
				"tools": [
					{"name": "go_symbol", "description": "Find Go symbols"},
					{"name": "search_file_local", "description": "Search files"}
				],
				"resources": [{"name": "config", "uri": "file:///config.json"}],
				"tool_scorecards": [
					{
						"tool_name": {
							"length": {"score": "pass", "justification": "Good length"},
							"uniqueness": {"score": "pass", "justification": "Unique"},
							"descriptiveness": {"score": "pass", "justification": "Clear"}
						},
						"tool_description": {
							"length": {"score": "pass", "justification": "Good"},
							"parameters": {"score": "pass", "justification": "Documented"},
							"examples": {"score": "fail", "justification": "Missing examples"}
						},
						"tool_input_schema": {
							"complexity": {"score": "pass", "justification": "Simple"},
							"parameters": {"score": "pass", "justification": "Clear"},
							"optionals": {"score": "pass", "justification": "Good defaults"},
							"constraints": {"score": "pass", "justification": "Valid"}
						},
						"tool_output_schema": {
							"complexity": {"score": "pass", "justification": "Simple"},
							"parameters": {"score": "pass", "justification": "Clear"},
							"optionals": {"score": "pass", "justification": "Good"},
							"constraints": {"score": "pass", "justification": "Valid"}
						}
					}
				]
			}`,
			wantErr:       false,
			wantServer:    "orca",
			wantToolCount: 2,
			wantCategs:    4,
		},
		{
			name: "minimal valid input",
			input: `{
				"initialize_result": {
					"protocolVersion": "2024-11-05",
					"serverInfo": {"name": "minimal"}
				},
				"tools": [],
				"resources": [],
				"tool_scorecards": []
			}`,
			wantErr:       false,
			wantServer:    "minimal",
			wantToolCount: 0,
			wantCategs:    4,
		},
		{
			name:    "invalid JSON",
			input:   `{not valid json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			pattern, err := adapter.Parse(reader)

			if tt.wantErr {
				if err == nil {
					t.Error("Parse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			report, ok := pattern.(*design.QualityReport)
			if !ok {
				t.Fatalf("Parse() returned wrong type: %T", pattern)
			}

			if report.ServerName != tt.wantServer {
				t.Errorf("ServerName = %q, want %q", report.ServerName, tt.wantServer)
			}

			if report.ToolCount != tt.wantToolCount {
				t.Errorf("ToolCount = %d, want %d", report.ToolCount, tt.wantToolCount)
			}

			if len(report.Categories) != tt.wantCategs {
				t.Errorf("got %d categories, want %d", len(report.Categories), tt.wantCategs)
			}
		})
	}
}

func TestMCPInterviewerAdapter_ScoreAggregation(t *testing.T) {
	adapter := &MCPInterviewerAdapter{}

	input := `{
		"initialize_result": {
			"protocolVersion": "2024-11-05",
			"serverInfo": {"name": "test"}
		},
		"tools": [
			{"name": "tool1", "description": "First tool"},
			{"name": "tool2", "description": "Second tool"}
		],
		"resources": [],
		"tool_scorecards": [
			{
				"tool_name": {
					"length": {"score": "pass", "justification": ""},
					"uniqueness": {"score": "pass", "justification": ""},
					"descriptiveness": {"score": "fail", "justification": "Unclear"}
				},
				"tool_description": {
					"length": {"score": "pass", "justification": ""},
					"parameters": {"score": "fail", "justification": "Missing"},
					"examples": {"score": "fail", "justification": "Missing"}
				},
				"tool_input_schema": {
					"complexity": {"score": "pass", "justification": ""},
					"parameters": {"score": "pass", "justification": ""},
					"optionals": {"score": "pass", "justification": ""},
					"constraints": {"score": "pass", "justification": ""}
				},
				"tool_output_schema": {
					"complexity": {"score": "pass", "justification": ""},
					"parameters": {"score": "pass", "justification": ""},
					"optionals": {"score": "pass", "justification": ""},
					"constraints": {"score": "pass", "justification": ""}
				}
			},
			{
				"tool_name": {
					"length": {"score": "pass", "justification": ""},
					"uniqueness": {"score": "pass", "justification": ""},
					"descriptiveness": {"score": "pass", "justification": ""}
				},
				"tool_description": {
					"length": {"score": "pass", "justification": ""},
					"parameters": {"score": "pass", "justification": ""},
					"examples": {"score": "fail", "justification": "Missing"}
				},
				"tool_input_schema": {
					"complexity": {"score": "pass", "justification": ""},
					"parameters": {"score": "pass", "justification": ""},
					"optionals": {"score": "pass", "justification": ""},
					"constraints": {"score": "pass", "justification": ""}
				},
				"tool_output_schema": {
					"complexity": {"score": "pass", "justification": ""},
					"parameters": {"score": "pass", "justification": ""},
					"optionals": {"score": "pass", "justification": ""},
					"constraints": {"score": "pass", "justification": ""}
				}
			}
		]
	}`

	reader := strings.NewReader(input)
	pattern, err := adapter.Parse(reader)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	report := pattern.(*design.QualityReport)

	// Check Name category: 6 total (3 per tool * 2 tools), 5 passed (1 fail for descriptiveness)
	nameCategory := report.Categories[0]
	if nameCategory.Name != "Name" {
		t.Errorf("Category[0].Name = %q, want %q", nameCategory.Name, "Name")
	}
	if nameCategory.Total != 6 {
		t.Errorf("Name.Total = %d, want 6", nameCategory.Total)
	}
	if nameCategory.Passed != 5 {
		t.Errorf("Name.Passed = %d, want 5", nameCategory.Passed)
	}

	// Check Description category: 6 total, 3 passed (3 failures: 2 examples + 1 parameters)
	descCategory := report.Categories[1]
	if descCategory.Total != 6 {
		t.Errorf("Description.Total = %d, want 6", descCategory.Total)
	}
	if descCategory.Passed != 3 {
		t.Errorf("Description.Passed = %d, want 3", descCategory.Passed)
	}

	// Check issues are tracked
	if len(report.Issues) == 0 {
		t.Error("Expected issues to be recorded for failures")
	}

	// Verify specific failure types are tracked
	foundExamplesIssue := false
	for _, issue := range report.Issues {
		if issue.Category == "description.examples" {
			foundExamplesIssue = true
			if issue.ToolCount != 2 {
				t.Errorf("description.examples ToolCount = %d, want 2", issue.ToolCount)
			}
		}
	}
	if !foundExamplesIssue {
		t.Error("Expected to find description.examples in issues")
	}
}

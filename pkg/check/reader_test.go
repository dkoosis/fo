package check

import (
	"testing"
)

func TestIsCheck(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{
			name: "valid lintkit-check",
			data: `{"$schema":"lintkit-check","tool":"filesize","status":"warn"}`,
			want: true,
		},
		{
			name: "lintkit-check with trailing text",
			data: `{"$schema":"lintkit-check","tool":"test","status":"pass"}
Some trailing output`,
			want: true,
		},
		{
			name: "sarif document",
			data: `{"version":"2.1.0","$schema":"https://sarif.schema"}`,
			want: false,
		},
		{
			name: "empty object",
			data: `{}`,
			want: false,
		},
		{
			name: "not json",
			data: `not json at all`,
			want: false,
		},
		{
			name: "empty",
			data: ``,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCheck([]byte(tt.data))
			if got != tt.want {
				t.Errorf("IsCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadBytes(t *testing.T) {
	data := []byte(`{
		"$schema": "lintkit-check",
		"tool": "filesize",
		"status": "warn",
		"summary": "3 red files",
		"metrics": [{"name": "red_files", "value": 3, "threshold": 0}],
		"items": [{"severity": "error", "label": "server.go", "value": "1847 LOC"}],
		"trend": [45000, 46200, 47100, 48230]
	}`)

	report, err := ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes() error = %v", err)
	}

	if report.Tool != "filesize" {
		t.Errorf("Tool = %q, want %q", report.Tool, "filesize")
	}
	if report.Status != "warn" {
		t.Errorf("Status = %q, want %q", report.Status, "warn")
	}
	if report.Summary != "3 red files" {
		t.Errorf("Summary = %q, want %q", report.Summary, "3 red files")
	}
	if len(report.Metrics) != 1 {
		t.Errorf("len(Metrics) = %d, want 1", len(report.Metrics))
	}
	if len(report.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(report.Items))
	}
	if len(report.Trend) != 4 {
		t.Errorf("len(Trend) = %d, want 4", len(report.Trend))
	}
}

func TestComputeStats(t *testing.T) {
	report := &Report{
		Schema: SchemaID,
		Items: []Item{
			{Severity: SeverityError, Label: "file1"},
			{Severity: SeverityError, Label: "file2"},
			{Severity: SeverityWarning, Label: "file3"},
			{Severity: SeverityInfo, Label: "file4"},
		},
	}

	stats := ComputeStats(report)

	if stats.TotalItems != 4 {
		t.Errorf("TotalItems = %d, want 4", stats.TotalItems)
	}
	if stats.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", stats.ErrorCount)
	}
	if stats.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", stats.WarningCount)
	}
	if stats.InfoCount != 1 {
		t.Errorf("InfoCount = %d, want 1", stats.InfoCount)
	}
}

func TestExtractCheck(t *testing.T) {
	data := []byte(`{"$schema":"lintkit-check","tool":"test"}trailing garbage`)

	extracted := ExtractCheck(data)
	if string(extracted) != `{"$schema":"lintkit-check","tool":"test"}` {
		t.Errorf("ExtractCheck() = %q, want JSON without trailing text", string(extracted))
	}
}

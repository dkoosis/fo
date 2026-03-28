package detect

import "testing"

func TestSniff_SARIF(t *testing.T) {
	input := `{"version":"2.1.0","$schema":"https://sarif.dev","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}`
	if got := Sniff([]byte(input)); got != SARIF {
		t.Errorf("expected SARIF, got %v", got)
	}
}

func TestSniff_GoTestJSON(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"start","Package":"example.com/pkg"}` + "\n"
	if got := Sniff([]byte(input)); got != GoTestJSON {
		t.Errorf("expected GoTestJSON, got %v", got)
	}
}

func TestSniff_GoTestJSON_OutputAction(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/pkg","Output":"=== RUN TestFoo\n"}` + "\n"
	if got := Sniff([]byte(input)); got != GoTestJSON {
		t.Errorf("expected GoTestJSON, got %v", got)
	}
}

func TestSniff_Empty(t *testing.T) {
	if got := Sniff([]byte("")); got != Unknown {
		t.Errorf("expected Unknown for empty, got %v", got)
	}
}

func TestSniff_PlainText(t *testing.T) {
	if got := Sniff([]byte("this is not json")); got != Unknown {
		t.Errorf("expected Unknown for plain text, got %v", got)
	}
}

func TestSniff_InvalidJSON(t *testing.T) {
	if got := Sniff([]byte("{invalid")); got != Unknown {
		t.Errorf("expected Unknown for invalid JSON, got %v", got)
	}
}

func TestSniff_LeadingWhitespace(t *testing.T) {
	input := `  {"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"x"}` + "\n"
	if got := Sniff([]byte(input)); got != GoTestJSON {
		t.Errorf("expected GoTestJSON with leading whitespace, got %v", got)
	}
}

func TestSniff_Report(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"sarif delimiter", "--- tool:vet format:sarif ---\n{\"version\":\"2.1.0\"}"},
		{"testjson delimiter", "--- tool:test format:testjson ---\n{\"Action\":\"run\"}"},
		{"sarif with status", "--- tool:vet format:sarif status:pass ---\nAll checks passed."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sniff([]byte(tt.input)); got != Report {
				t.Errorf("Sniff() = %v, want Report", got)
			}
		})
	}
}

func TestSniff_SARIFWithTrailingText(t *testing.T) {
	// golangci-lint v2 appends a text summary after the SARIF JSON document
	input := `{"version":"2.1.0","$schema":"https://sarif.dev","runs":[{"tool":{"driver":{"name":"golangci-lint"}},"results":[]}]}
1 issues:
* gocognit: 1
`
	if got := Sniff([]byte(input)); got != SARIF {
		t.Errorf("expected SARIF with trailing text, got %v", got)
	}
}

func TestSniff_NonSARIFVersion(t *testing.T) {
	// JSON with version field but not SARIF 2.1.0 should not match
	input := `{"version":"1.0","runs":[{}]}`
	if got := Sniff([]byte(input)); got != Unknown {
		t.Errorf("expected Unknown for non-SARIF version, got %v", got)
	}
}

func TestFormatString(t *testing.T) {
	tests := []struct {
		f    Format
		want string
	}{
		{Unknown, "Unknown"},
		{SARIF, "SARIF"},
		{GoTestJSON, "GoTestJSON"},
		{Report, "Report"},
		{Format(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.want {
			t.Errorf("Format(%d).String() = %q, want %q", tt.f, got, tt.want)
		}
	}
}

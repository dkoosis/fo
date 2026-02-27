package detect

import "testing"

func TestSniff_SARIF(t *testing.T) {
	input := `{"version":"2.1.0","$schema":"https://sarif.dev","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}`
	if got := Sniff([]byte(input)); got != SARIF {
		t.Errorf("expected SARIF, got %d", got)
	}
}

func TestSniff_GoTestJSON(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"start","Package":"example.com/pkg"}` + "\n"
	if got := Sniff([]byte(input)); got != GoTestJSON {
		t.Errorf("expected GoTestJSON, got %d", got)
	}
}

func TestSniff_GoTestJSON_OutputAction(t *testing.T) {
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/pkg","Output":"=== RUN TestFoo\n"}` + "\n"
	if got := Sniff([]byte(input)); got != GoTestJSON {
		t.Errorf("expected GoTestJSON, got %d", got)
	}
}

func TestSniff_Empty(t *testing.T) {
	if got := Sniff([]byte("")); got != Unknown {
		t.Errorf("expected Unknown for empty, got %d", got)
	}
}

func TestSniff_PlainText(t *testing.T) {
	if got := Sniff([]byte("this is not json")); got != Unknown {
		t.Errorf("expected Unknown for plain text, got %d", got)
	}
}

func TestSniff_InvalidJSON(t *testing.T) {
	if got := Sniff([]byte("{invalid")); got != Unknown {
		t.Errorf("expected Unknown for invalid JSON, got %d", got)
	}
}

func TestSniff_LeadingWhitespace(t *testing.T) {
	input := `  {"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"x"}` + "\n"
	if got := Sniff([]byte(input)); got != GoTestJSON {
		t.Errorf("expected GoTestJSON with leading whitespace, got %d", got)
	}
}

func TestSniff_Report(t *testing.T) {
	input := []byte("--- tool:vet format:sarif ---\n{\"version\":\"2.1.0\"}")
	got := Sniff(input)
	if got != Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportWithStatus(t *testing.T) {
	input := []byte("--- tool:arch format:text status:pass ---\nAll checks passed.")
	got := Sniff(input)
	if got != Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportMetricsFormat(t *testing.T) {
	input := []byte("--- tool:eval format:metrics ---\n{\"scope\":\"86 queries\"}")
	got := Sniff(input)
	if got != Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportArchlintFormat(t *testing.T) {
	input := []byte("--- tool:arch format:archlint ---\n{}")
	got := Sniff(input)
	if got != Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportJscpdFormat(t *testing.T) {
	input := []byte("--- tool:dupl format:jscpd ---\n{}")
	got := Sniff(input)
	if got != Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

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

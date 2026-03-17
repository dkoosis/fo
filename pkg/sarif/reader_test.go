package sarif

import (
	"strings"
	"testing"
)

const wantVersion = "2.1.0"

// minimalSARIF is the smallest valid SARIF document.
const minimalSARIF = `{"version":"` + wantVersion + `","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}`

func TestRead_ValidDocument(t *testing.T) {
	doc, err := read(strings.NewReader(minimalSARIF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestRead_ValidWithTrailingWhitespace(t *testing.T) {
	input := minimalSARIF + "   \n\t\n  "
	doc, err := read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("trailing whitespace should be accepted, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestRead_TrailingPlainText(t *testing.T) {
	// golangci-lint v2 appends a text summary after SARIF — tolerate it.
	input := minimalSARIF + "\n1 issues:\n* gocognit: 1\n"
	doc, err := read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("trailing plain text should be accepted, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestRead_TrailingJSONObject(t *testing.T) {
	// Trailing JSON is tolerated — we only care about the first SARIF document.
	input := minimalSARIF + `{"extra":"object"}`
	doc, err := read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("trailing JSON should be accepted, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestRead_InvalidJSON(t *testing.T) {
	_, err := read(strings.NewReader(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestRead_MissingVersion(t *testing.T) {
	_, err := read(strings.NewReader(`{"runs":[]}`))
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
}

func TestReadBytes_ValidDocument(t *testing.T) {
	doc, err := ReadBytes([]byte(minimalSARIF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestReadBytes_TrailingJSON(t *testing.T) {
	input := minimalSARIF + `{"extra":true}`
	doc, err := ReadBytes([]byte(input))
	if err != nil {
		t.Fatalf("trailing JSON should be accepted via ReadBytes, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestReadBytes_TrailingText(t *testing.T) {
	input := minimalSARIF + "\n1 issues:\n* gocognit: 1\n"
	doc, err := ReadBytes([]byte(input))
	if err != nil {
		t.Fatalf("trailing text should be accepted via ReadBytes, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

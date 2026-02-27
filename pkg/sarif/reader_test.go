package sarif

import (
	"strings"
	"testing"
)

const wantVersion = "2.1.0"

// minimalSARIF is the smallest valid SARIF document.
const minimalSARIF = `{"version":"` + wantVersion + `","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}`

func TestRead_ValidDocument(t *testing.T) {
	doc, err := Read(strings.NewReader(minimalSARIF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestRead_ValidWithTrailingWhitespace(t *testing.T) {
	input := minimalSARIF + "   \n\t\n  "
	doc, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("trailing whitespace should be accepted, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
}

func TestRead_TrailingGarbageText(t *testing.T) {
	input := minimalSARIF + `garbage`
	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for trailing garbage text, got nil")
	}
	if !strings.Contains(err.Error(), "trailing data") {
		t.Errorf("expected trailing data error, got: %v", err)
	}
}

func TestRead_TrailingJSONObject(t *testing.T) {
	input := minimalSARIF + `{"extra":"object"}`
	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for trailing JSON object, got nil")
	}
	if !strings.Contains(err.Error(), "trailing data") {
		t.Errorf("expected trailing data error, got: %v", err)
	}
}

func TestRead_InvalidJSON(t *testing.T) {
	_, err := Read(strings.NewReader(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestRead_MissingVersion(t *testing.T) {
	_, err := Read(strings.NewReader(`{"runs":[]}`))
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

func TestReadBytes_TrailingGarbage(t *testing.T) {
	input := minimalSARIF + `{"extra":true}`
	_, err := ReadBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for trailing garbage via ReadBytes, got nil")
	}
}

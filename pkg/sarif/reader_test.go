package sarif

import (
	"errors"
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
	assertMinimalDoc(t, doc)
}

// assertMinimalDoc verifies the full structure parsed out of minimalSARIF,
// not just Version — a parser that returned a zero-value doc with only
// Version set would otherwise pass.
func assertMinimalDoc(t *testing.T, doc *Document) {
	t.Helper()
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(doc.Runs))
	}
	if got := doc.Runs[0].Tool.Driver.Name; got != "test" {
		t.Errorf("expected driver name %q, got %q", "test", got)
	}
	if got := len(doc.Runs[0].Results); got != 0 {
		t.Errorf("expected 0 results, got %d", got)
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

func TestRead_TrailingPlainText(t *testing.T) {
	// golangci-lint v2 appends a text summary after SARIF — tolerate it.
	input := minimalSARIF + "\n1 issues:\n* gocognit: 1\n"
	doc, err := Read(strings.NewReader(input))
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
	doc, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("trailing JSON should be accepted, got error: %v", err)
	}
	if doc.Version != wantVersion {
		t.Errorf("expected version %s, got %s", wantVersion, doc.Version)
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
	assertMinimalDoc(t, doc)
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

// TestReadBytes_DepthBomb verifies the depth guard rejects pathologically
// nested input (#269) before the recursive Decode can overflow the stack.
func TestReadBytes_DepthBomb(t *testing.T) {
	depth := maxNestingDepth + 50
	bomb := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	_, err := ReadBytes([]byte(bomb))
	if !errors.Is(err, ErrNestingTooDeep) {
		t.Fatalf("expected ErrNestingTooDeep, got %v", err)
	}
}

// TestReadBytes_DeepButBounded confirms the guard does not reject documents
// nested below the limit — only the depth-bomb is rejected, not valid SARIF.
func TestReadBytes_DeepButBounded(t *testing.T) {
	depth := maxNestingDepth - 1
	nested := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	if err := checkDepth([]byte(nested)); err != nil {
		t.Fatalf("depth %d is under the limit and must pass, got %v", depth, err)
	}
}

package report

import (
	"errors"
	"testing"
)

func TestParse_SingleSARIF(t *testing.T) {
	input := "--- tool:lint format:sarif ---\n{\"version\":\"2.1.0\"}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1", len(sections))
	}
	if sections[0].Tool != "lint" {
		t.Errorf("tool = %q, want lint", sections[0].Tool)
	}
	if sections[0].Format != "sarif" {
		t.Errorf("format = %q, want sarif", sections[0].Format)
	}
}

func TestParse_MultipleSections(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n{}\n--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n--- tool:arch format:text status:pass ---\nOK\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 3 {
		t.Fatalf("got %d sections, want 3", len(sections))
	}
	if sections[2].Status != "pass" {
		t.Errorf("status = %q, want pass", sections[2].Status)
	}
}

func TestParse_EmptySection(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n--- tool:lint format:sarif ---\n{}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if len(sections[0].Content) != 0 {
		t.Errorf("section 0 content should be empty, got %d bytes", len(sections[0].Content))
	}
}

func TestParse_PreservesTrailingNewlineInContent(t *testing.T) {
	input := "--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	content := string(sections[0].Content)
	if content != "{\"Action\":\"pass\"}" {
		t.Errorf("content = %q", content)
	}
}

func TestParse_NoDelimiters(t *testing.T) {
	_, err := Parse([]byte("just some random text\n"))
	if !errors.Is(err, ErrNoSections) {
		t.Errorf("got %v, want ErrNoSections", err)
	}
}

func TestParse_DiscardsLinesBeforeFirstDelimiter(t *testing.T) {
	input := "preamble junk\nmore junk\n--- tool:lint format:sarif ---\n{}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1", len(sections))
	}
	if string(sections[0].Content) != "{}" {
		t.Errorf("content = %q, want %q", sections[0].Content, "{}")
	}
}

func TestIsDelimiter(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"--- tool:lint format:sarif ---", true},
		{"--- tool:test format:testjson ---", true},
		{"--- tool:arch format:text status:pass ---", true},
		{"--- tool:m format:metrics ---", true},
		{"not a delimiter", false},
		{"--- tool: format:sarif ---", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsDelimiter([]byte(tt.line)); got != tt.want {
			t.Errorf("IsDelimiter(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

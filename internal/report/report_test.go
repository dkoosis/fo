package report

import (
	"errors"
	"fmt"
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
	input := "--- tool:vet format:sarif ---\n{}\n--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
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
		{"--- tool:lint format:sarif status:ok ---", true},
		{"--- tool:lint format:sarif status:clean ---", true},
		{"--- tool:lint format:sarif status:error ---", true},
		{"--- tool:lint format:sarif status:skipped ---", true},
		{"--- tool:lint format:sarif status:timeout ---", true},
		{"--- tool:lint format:sarif status:partial ---", true},
		{"--- tool:arch format:text status:pass ---", false},
		{"--- tool:lint format:sarif status:pass ---", false},
		{"--- tool:lint format:sarif status:bogus ---", false},
		{"--- tool:m format:metrics ---", false},
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

func TestParse_CRLFLineEndings(t *testing.T) {
	input := "--- tool:lint format:sarif ---\r\n{\"version\":\"2.1.0\"}\r\n--- tool:test format:testjson ---\r\n{\"Action\":\"pass\"}\r\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if sections[0].Tool != "lint" {
		t.Errorf("tool = %q, want lint", sections[0].Tool)
	}
	if sections[0].Format != "sarif" {
		t.Errorf("format = %q, want sarif", sections[0].Format)
	}
	if string(sections[0].Content) != "{\"version\":\"2.1.0\"}" {
		t.Errorf("content = %q, want %q", sections[0].Content, "{\"version\":\"2.1.0\"}")
	}
	if sections[1].Tool != "test" {
		t.Errorf("tool = %q, want test", sections[1].Tool)
	}
}

func TestParse_MixedLineEndings(t *testing.T) {
	// First section uses CRLF, second uses LF
	input := "--- tool:vet format:sarif ---\r\n{}\r\n--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if sections[0].Tool != "vet" {
		t.Errorf("tool = %q, want vet", sections[0].Tool)
	}
	if sections[1].Tool != "test" {
		t.Errorf("tool = %q, want test", sections[1].Tool)
	}
}

func TestParse_StatusAttribute(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		status Status
	}{
		{"status ok", "--- tool:lint format:sarif status:ok ---\n{}\n", StatusOK},
		{"status clean", "--- tool:lint format:sarif status:clean ---\n{}\n", StatusClean},
		{"status error", "--- tool:lint format:sarif status:error ---\n{}\n", StatusError},
		{"status skipped", "--- tool:lint format:sarif status:skipped ---\n\n", StatusSkipped},
		{"status timeout", "--- tool:lint format:sarif status:timeout ---\n\n", StatusTimeout},
		{"status partial", "--- tool:lint format:sarif status:partial ---\n{}\n", StatusPartial},
		{"status absent (backward-compat)", "--- tool:lint format:sarif ---\n{}\n", Status("")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if len(sections) != 1 {
				t.Fatalf("got %d sections", len(sections))
			}
			if sections[0].Status != tt.status {
				t.Errorf("status = %q, want %q", sections[0].Status, tt.status)
			}
		})
	}
}

func TestParse_InvalidStatusFallsThrough(t *testing.T) {
	// A delimiter with an unknown status token does not match the regex — so
	// it is treated as preamble and discarded. This keeps the protocol strict.
	input := "--- tool:lint format:sarif status:wat ---\n{}\n"
	_, err := Parse([]byte(input))
	if !errors.Is(err, ErrNoSections) {
		t.Errorf("want ErrNoSections, got %v", err)
	}
}

func TestIsValidStatus(t *testing.T) {
	for _, s := range []string{"ok", "clean", "partial", "timeout", "skipped", "error"} {
		if !IsValidStatus(s) {
			t.Errorf("IsValidStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "pass", "fail", "wat"} {
		if IsValidStatus(s) {
			t.Errorf("IsValidStatus(%q) = true, want false", s)
		}
	}
}

func TestParse_OldFormatsNotRecognized(t *testing.T) {
	for _, format := range []string{"text", "metrics", "archlint", "jscpd"} {
		input := []byte(fmt.Sprintf("--- tool:x format:%s ---\ncontent\n", format))
		sections, err := Parse(input)
		// Old format delimiter is not recognized by the narrowed regex.
		// It becomes preamble, and with no valid delimiters, Parse returns ErrNoSections.
		if err == nil {
			t.Errorf("format:%s — expected ErrNoSections, got %d sections", format, len(sections))
		}
		if !errors.Is(err, ErrNoSections) {
			t.Errorf("format:%s — expected ErrNoSections, got %v", format, err)
		}
	}
}

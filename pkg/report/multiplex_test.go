package report

import (
	"errors"
	"strings"
	"testing"
)

func TestIsDelimiter(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"--- tool:vet format:sarif ---", true},
		{"--- tool:test format:testjson ---", true},
		{"--- tool:go-vet format:sarif ---", true},
		{"--- tool:vet format:json ---", false},
		{"--- tool:vet format:sarif", false},
		{"# heading", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsDelimiter([]byte(c.line)); got != c.want {
			t.Errorf("IsDelimiter(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestHasDelimiter(t *testing.T) {
	cases := []struct {
		data string
		want bool
	}{
		{"--- tool:vet format:sarif ---\nfoo", true},
		{"   \n\n--- tool:vet format:sarif ---\n", true},
		{"{\"version\":\"2.1.0\"}", false},
		{"", false},
	}
	for _, c := range cases {
		if got := HasDelimiter([]byte(c.data)); got != c.want {
			t.Errorf("HasDelimiter(%q) = %v, want %v", c.data, got, c.want)
		}
	}
}

func TestParseSections(t *testing.T) {
	input := "preamble line\n" +
		"--- tool:vet format:sarif ---\n" +
		"vet body line 1\n" +
		"vet body line 2\n" +
		"--- tool:test format:testjson ---\n" +
		"{\"Action\":\"pass\"}\n"

	got, prelude, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("ParseSections err = %v", err)
	}
	if string(prelude) != "preamble line" {
		t.Errorf("prelude = %q, want %q", prelude, "preamble line")
	}
	if len(got) != 2 {
		t.Fatalf("got %d sections, want 2", len(got))
	}
	if got[0].Tool != "vet" || got[0].Format != "sarif" {
		t.Errorf("section[0] = %+v", got[0])
	}
	if string(got[0].Content) != "vet body line 1\nvet body line 2" {
		t.Errorf("section[0].Content = %q", got[0].Content)
	}
	if got[1].Tool != "test" || got[1].Format != "testjson" {
		t.Errorf("section[1] = %+v", got[1])
	}
	if string(got[1].Content) != `{"Action":"pass"}` {
		t.Errorf("section[1].Content = %q", got[1].Content)
	}
}

func TestParseSectionsCRLF(t *testing.T) {
	input := "--- tool:vet format:sarif ---\r\nbody\r\n"
	got, _, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 || string(got[0].Content) != "body" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSectionsEmpty(t *testing.T) {
	if _, _, err := ParseSections([]byte("no delimiters here\nat all\n")); !errors.Is(err, ErrNoSections) {
		t.Errorf("err = %v, want ErrNoSections", err)
	}
}

func TestParseSectionsEmptyContent(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n--- tool:test format:testjson ---\nbody\n"
	got, _, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sections, want 2", len(got))
	}
	if len(got[0].Content) != 0 {
		t.Errorf("section[0].Content = %q, want empty", got[0].Content)
	}
}

func TestParseSectionsPreludeSurfaced(t *testing.T) {
	input := "stray banner\nfrom wrapper\n--- tool:vet format:sarif ---\nbody\n"
	got, prelude, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sections, want 1", len(got))
	}
	want := "stray banner\nfrom wrapper"
	if string(prelude) != want {
		t.Errorf("prelude = %q, want %q", prelude, want)
	}
}

func TestParseSectionsWhitespacePreludeSilent(t *testing.T) {
	input := "\n  \n\t\n--- tool:vet format:sarif ---\nbody\n"
	got, prelude, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sections, want 1", len(got))
	}
	if prelude != nil {
		t.Errorf("prelude = %q, want nil", prelude)
	}
}

func TestParseSections_StatusAttribute(t *testing.T) {
	cases := []struct {
		line       string
		wantStatus string
	}{
		{"--- tool:vet format:sarif status:ok ---", "ok"},
		{"--- tool:vet format:sarif status:clean ---", "clean"},
		{"--- tool:vet format:sarif status:timeout ---", "timeout"},
		{"--- tool:vet format:sarif status:error ---", "error"},
		{"--- tool:vet format:sarif status:partial ---", "partial"},
		{"--- tool:vet format:sarif status:skipped ---", "skipped"},
		{"--- tool:vet format:sarif ---", ""},
	}
	for _, c := range cases {
		input := c.line + "\nbody\n"
		got, _, err := ParseSections([]byte(input))
		if err != nil {
			t.Errorf("%q: ParseSections err = %v", c.line, err)
			continue
		}
		if len(got) != 1 {
			t.Errorf("%q: got %d sections, want 1", c.line, len(got))
			continue
		}
		if got[0].Status != c.wantStatus {
			t.Errorf("%q: Status = %q, want %q", c.line, got[0].Status, c.wantStatus)
		}
	}
}

func TestHasDelimiter_UnknownFormatRoutesToMultiplexer(t *testing.T) {
	// Shape-only delimiters should be detected so the multiplexer can surface
	// a precise error instead of falling through to 'unrecognized input'.
	if !HasDelimiter([]byte("--- tool:vet format:text ---\nbody\n")) {
		t.Errorf("HasDelimiter should match shape-only delimiter with unknown format")
	}
}

func TestParseSections_UnknownFormat(t *testing.T) {
	input := "--- tool:vet format:sarif ---\nok body\n" +
		"--- tool:build format:text ---\nbuild error here\n"
	_, _, err := ParseSections([]byte(input))
	var ufe *UnknownFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("err = %v, want UnknownFormatError", err)
	}
	if ufe.SectionIndex != 2 {
		t.Errorf("SectionIndex = %d, want 2", ufe.SectionIndex)
	}
	if ufe.Tool != "build" || ufe.Format != "text" {
		t.Errorf("Tool=%q Format=%q, want build/text", ufe.Tool, ufe.Format)
	}
	msg := ufe.Error()
	if !strings.Contains(msg, "sarif") || !strings.Contains(msg, "testjson") {
		t.Errorf("error message %q should list supported formats", msg)
	}
	if !strings.Contains(msg, `"text"`) {
		t.Errorf("error message %q should quote offending format", msg)
	}
}

func TestParseSections_UnknownFormatFirstSection(t *testing.T) {
	_, _, err := ParseSections([]byte("--- tool:build format:text ---\nbody\n"))
	var ufe *UnknownFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("err = %v, want UnknownFormatError", err)
	}
	if ufe.SectionIndex != 1 {
		t.Errorf("SectionIndex = %d, want 1", ufe.SectionIndex)
	}
}

func TestIsDelimiter_WithStatus(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"--- tool:vet format:sarif status:ok ---", true},
		{"--- tool:vet format:sarif status:timeout ---", true},
		{"--- tool:vet format:sarif status:error ---", true},
		{"--- tool:vet format:sarif status: ---", false},
	}
	for _, c := range cases {
		if got := IsDelimiter([]byte(c.line)); got != c.want {
			t.Errorf("IsDelimiter(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

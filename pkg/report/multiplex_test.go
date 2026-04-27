package report

import (
	"errors"
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

	got, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("ParseSections err = %v", err)
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
	got, err := ParseSections([]byte(input))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 || string(got[0].Content) != "body" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSectionsEmpty(t *testing.T) {
	if _, err := ParseSections([]byte("no delimiters here\nat all\n")); !errors.Is(err, ErrNoSections) {
		t.Errorf("err = %v, want ErrNoSections", err)
	}
}

func TestParseSectionsEmptyContent(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n--- tool:test format:testjson ---\nbody\n"
	got, err := ParseSections([]byte(input))
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

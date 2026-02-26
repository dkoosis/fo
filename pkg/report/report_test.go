package report

import "testing"

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

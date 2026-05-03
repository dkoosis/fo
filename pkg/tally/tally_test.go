package tally

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestIsHeader(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# fo:tally\n14 a\n", true},
		{"   # fo:tally tool=x\n", true},
		{"\n# fo:tally\n", true},
		{"# fo: tally\n", false},
		{"foo\n# fo:tally\n", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsHeader([]byte(c.in)); got != c.want {
			t.Errorf("IsHeader(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParse_basic(t *testing.T) {
	in := `# fo:tally tool=dk-types
14332 log.friction
2578 journal.day
701 log.session
`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Tool != "dk-types" {
		t.Errorf("Tool = %q, want %q", got.Tool, "dk-types")
	}
	if len(got.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(got.Rows))
	}
	if got.Rows[0].Value != 14332 || got.Rows[0].Label != "log.friction" {
		t.Errorf("row[0] = %+v", got.Rows[0])
	}
}

func TestParse_uniq_c_format(t *testing.T) {
	// `sort | uniq -c` right-aligns counts; tally must accept leading ws.
	in := "# fo:tally\n  14332 log.friction\n   2578 journal.day\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(got.Rows))
	}
	if got.Rows[0].Value != 14332 {
		t.Errorf("row[0].Value = %v, want 14332", got.Rows[0].Value)
	}
}

func TestParse_labelWithSpaces(t *testing.T) {
	in := "# fo:tally\n5 entity.person\n3 reference article\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Rows[1].Label != "reference article" {
		t.Errorf("row[1].Label = %q, want %q", got.Rows[1].Label, "reference article")
	}
}

func TestParse_blankAndComment(t *testing.T) {
	in := "# fo:tally\n\n# a comment\n5 a\n\n3 b\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(got.Rows))
	}
}

func TestParse_missingHeader(t *testing.T) {
	in := "5 a\n3 b\n"
	_, err := Parse(strings.NewReader(in))
	if !errors.Is(err, ErrNoHeader) {
		t.Errorf("err = %v, want ErrNoHeader", err)
	}
}

func TestParse_noRows(t *testing.T) {
	in := "# fo:tally\n\n# nothing here\n"
	_, err := Parse(strings.NewReader(in))
	if !errors.Is(err, ErrNoRows) {
		t.Errorf("err = %v, want ErrNoRows", err)
	}
}

func TestParse_malformedRow(t *testing.T) {
	in := "# fo:tally\nnotanumber a\n"
	_, err := Parse(strings.NewReader(in))
	if !errors.Is(err, ErrMalformedRow) || !strings.Contains(err.Error(), "non-numeric") {
		t.Errorf("err = %v, want ErrMalformedRow with non-numeric detail", err)
	}
}

func TestParse_missingLabel(t *testing.T) {
	in := "# fo:tally\n5\n"
	_, err := Parse(strings.NewReader(in))
	if !errors.Is(err, ErrMalformedRow) || !strings.Contains(err.Error(), "expected '<count> <label>'") {
		t.Errorf("err = %v, want ErrMalformedRow with shape detail", err)
	}
}

func TestToLeaderboard(t *testing.T) {
	tly := Tally{Rows: []Row{
		{Label: "a", Value: 10},
		{Label: "b", Value: 5},
		{Label: "c", Value: 1},
	}}
	lb := tly.ToLeaderboard()
	if lb.Total != 16 {
		t.Errorf("Total = %v, want 16", lb.Total)
	}
	if len(lb.Rows) != 3 || lb.Rows[0].Label != "a" {
		t.Errorf("rows = %+v", lb.Rows)
	}
}

func TestRenderLLM(t *testing.T) {
	tly := Tally{Rows: []Row{
		{Label: "log.friction", Value: 14332},
		{Label: "journal.day", Value: 2578},
	}}
	var buf bytes.Buffer
	if err := tly.RenderLLM(&buf); err != nil {
		t.Fatalf("RenderLLM: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "log.friction") || !strings.Contains(out, "14332") {
		t.Errorf("output missing data: %q", out)
	}
}

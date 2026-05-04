package status

import (
	"errors"
	"strings"
	"testing"
)

func TestIsHeader(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# fo:status\nok foo\n", true},
		{"# fo:status tool=doctor\n", true},
		{"  # fo:status\n", true},
		{"# fo:tally\n", false},
		{"ok foo\n", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsHeader([]byte(c.in)); got != c.want {
			t.Errorf("IsHeader(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParse_basic(t *testing.T) {
	in := strings.NewReader("# fo:status tool=doctor\nok\tenv loaded\nfail\tdolt missing\t\tnot-installed\n")
	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Tool != "doctor" {
		t.Errorf("Tool = %q", s.Tool)
	}
	if len(s.Rows) != 2 {
		t.Fatalf("rows = %d", len(s.Rows))
	}
	if s.Rows[0] != (Row{State: StateOK, Label: "env loaded"}) {
		t.Errorf("row0 = %+v", s.Rows[0])
	}
	if s.Rows[1].State != StateFail || s.Rows[1].Label != "dolt missing" || s.Rows[1].Note != "not-installed" {
		t.Errorf("row1 = %+v", s.Rows[1])
	}
}

func TestParse_spaceOnly(t *testing.T) {
	in := strings.NewReader("# fo:status\nok build green\n")
	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Rows[0].Label != "build green" || s.Rows[0].Value != "" || s.Rows[0].Note != "" {
		t.Errorf("row = %+v", s.Rows[0])
	}
}

func TestParse_valueAndNote(t *testing.T) {
	in := strings.NewReader("# fo:status\nok\tbuild\t2.3s\tgreen\n")
	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Rows[0].Value != "2.3s" || s.Rows[0].Note != "green" {
		t.Errorf("row = %+v", s.Rows[0])
	}
}

func TestParse_errors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"no header", "ok foo\n", ErrNoHeader},
		{"no rows", "# fo:status\n", ErrNoRows},
		{"bad state", "# fo:status\nbogus foo\n", ErrBadState},
		{"missing label", "# fo:status\nok\n", ErrMalformedRow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(c.in))
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want Is %v", err, c.want)
			}
		})
	}
}

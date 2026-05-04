package metrics

import (
	"errors"
	"strings"
	"testing"
)

func TestIsHeader(t *testing.T) {
	if !IsHeader([]byte("# fo:metrics\n")) {
		t.Error("expected header detected")
	}
	if IsHeader([]byte("# fo:status\n")) {
		t.Error("status should not match")
	}
}

func TestParse_basic(t *testing.T) {
	in := strings.NewReader("# fo:metrics tool=cover\npkg/x 87.3 %\npkg/y 100 %\n")
	m, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Tool != "cover" {
		t.Errorf("tool = %q", m.Tool)
	}
	if len(m.Rows) != 2 {
		t.Fatalf("rows = %d", len(m.Rows))
	}
	if m.Rows[0] != (Row{Key: "pkg/x", Value: 87.3, Unit: "%"}) {
		t.Errorf("row0 = %+v", m.Rows[0])
	}
}

func TestParse_noUnit(t *testing.T) {
	m, err := Parse(strings.NewReader("# fo:metrics\nbuild_time 2.3\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Rows[0].Unit != "" || m.Rows[0].Value != 2.3 {
		t.Errorf("row = %+v", m.Rows[0])
	}
}

func TestParse_errors(t *testing.T) {
	cases := []struct {
		in   string
		want error
	}{
		{"x 1\n", ErrNoHeader},
		{"# fo:metrics\n", ErrNoRows},
		{"# fo:metrics\nbad\n", ErrMalformedRow},
		{"# fo:metrics\nx not-a-number\n", ErrMalformedRow},
	}
	for _, c := range cases {
		_, err := Parse(strings.NewReader(c.in))
		if !errors.Is(err, c.want) {
			t.Errorf("err = %v, want Is %v", err, c.want)
		}
	}
}

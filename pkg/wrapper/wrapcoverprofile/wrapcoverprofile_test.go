package wrapcoverprofile

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvert_EmitsUncoveredBlocksOnly(t *testing.T) {
	in := "mode: set\n" +
		"github.com/x/y/foo.go:12.13,15.4 3 0\n" + // uncovered
		"github.com/x/y/foo.go:20.2,22.10 2 1\n" + // covered → skipped
		"github.com/x/y/bar.go:5.1,7.2 1 0\n" // uncovered
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		`"ruleId": "uncovered"`,
		`"level": "note"`,
		"foo.go",
		"bar.go",
		"3 statement(s) uncovered",
		"1 statement(s) uncovered",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in SARIF:\n%s", want, got)
		}
	}
	// The covered block (foo.go:20) must not appear.
	if strings.Contains(got, "2 statement(s) uncovered") {
		t.Errorf("covered block leaked into output:\n%s", got)
	}
	if n := strings.Count(got, `"ruleId": "uncovered"`); n != 2 {
		t.Errorf("want 2 uncovered findings, got %d", n)
	}
}

func TestConvert_StartLineAndCol(t *testing.T) {
	in := "mode: count\ngithub.com/x/y/foo.go:12.13,15.4 3 0\n"
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"startLine": 12`) || !strings.Contains(got, `"startColumn": 13`) {
		t.Errorf("start position wrong:\n%s", got)
	}
}

func TestConvert_EmptyProfile(t *testing.T) {
	var out bytes.Buffer
	if err := Convert(strings.NewReader("mode: set\n"), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	// Valid SARIF carrying no uncovered findings.
	if strings.Contains(out.String(), ruleUncovered) {
		t.Errorf("empty profile should yield no uncovered findings:\n%s", out.String())
	}
}

func TestParseBlock(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantOK    bool
		file      string
		startLine int
		startCol  int
		stmts     int
		count     int
	}{
		{"uncovered", "github.com/x/y/foo.go:12.13,15.4 3 0", true, "github.com/x/y/foo.go", 12, 13, 3, 0},
		{"covered", "a.go:1.1,2.2 1 5", true, "a.go", 1, 1, 1, 5},
		{"mode header", "mode: set", false, "", 0, 0, 0, 0},
		{"blank", "", false, "", 0, 0, 0, 0},
		{"garbage", "not a coverprofile line", false, "", 0, 0, 0, 0},
		{"missing count", "a.go:1.1,2.2 1", false, "", 0, 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file, sl, sc, stmts, count, ok := parseBlock(c.line)
			if ok != c.wantOK {
				t.Fatalf("ok=%v want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if file != c.file || sl != c.startLine || sc != c.startCol || stmts != c.stmts || count != c.count {
				t.Errorf("got (%q,%d,%d,%d,%d) want (%q,%d,%d,%d,%d)",
					file, sl, sc, stmts, count, c.file, c.startLine, c.startCol, c.stmts, c.count)
			}
		})
	}
}

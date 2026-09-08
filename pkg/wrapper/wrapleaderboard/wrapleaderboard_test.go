package wrapleaderboard

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/lineread"
	"github.com/dkoosis/fo/pkg/tally"
)

func TestConvert_basic(t *testing.T) {
	in := "5 alpha\n3 beta\n"
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out, Opts{Tool: "x"}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "# fo:tally tool=x\n") {
		t.Errorf("missing header: %q", got)
	}
	if !tally.IsHeader(out.Bytes()) {
		t.Errorf("output not detected as tally")
	}
	// Round-trip through tally.Parse.
	parsed, err := tally.Parse(strings.NewReader(got), nil)
	if err != nil {
		t.Fatalf("round-trip Parse: %v", err)
	}
	if len(parsed.Rows) != 2 || parsed.Rows[0].Label != "alpha" {
		t.Errorf("parsed = %+v", parsed)
	}
}

func TestConvert_uniqCFormat(t *testing.T) {
	// `sort | uniq -c` output has leading whitespace + right-aligned counts.
	in := "  14332 log.friction\n   2578 journal.day\n"
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out, Opts{}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	parsed, err := tally.Parse(strings.NewReader(out.String()), nil)
	if err != nil {
		t.Fatalf("round-trip Parse: %v", err)
	}
	if len(parsed.Rows) != 2 || parsed.Rows[0].Value != 14332 {
		t.Errorf("rows = %+v", parsed.Rows)
	}
}

func TestConvert_noTool(t *testing.T) {
	var out bytes.Buffer
	if err := Convert(strings.NewReader("5 a\n"), &out, Opts{}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.HasPrefix(out.String(), "# fo:tally\n") {
		t.Errorf("header without tool flag wrong: %q", out.String())
	}
}

func TestConvert_emptyInput(t *testing.T) {
	var out bytes.Buffer
	err := Convert(strings.NewReader(""), &out, Opts{})
	if !errors.Is(err, errNoRows) {
		t.Errorf("err = %v, want errNoRows", err)
	}
}

func TestConvert_commentsAndBlanks(t *testing.T) {
	in := "\n# header comment\n5 a\n\n# inline\n3 b\n"
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out, Opts{}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	parsed, err := tally.Parse(strings.NewReader(out.String()), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(parsed.Rows) != 2 {
		t.Errorf("rows = %d, want 2", len(parsed.Rows))
	}
}

func TestConvert_malformed(t *testing.T) {
	in := "abc def\n"
	var out bytes.Buffer
	err := Convert(strings.NewReader(in), &out, Opts{})
	if !errors.Is(err, errMalformedRow) || !strings.Contains(err.Error(), "non-numeric") {
		t.Errorf("err = %v, want errMalformedRow with non-numeric detail", err)
	}
}

// fo-n25.6: oversize-line drops must surface as a stderr warning written
// to the injected Opts.Stderr, mirroring wrapdiag.DiagOpts.Stderr.
func TestConvert_OversizeLineWarnsInjectedStderr(t *testing.T) {
	huge := strings.Repeat("Z", lineread.MaxLineLen+1024)
	in := huge + "\n5 kept\n"

	var out, errBuf bytes.Buffer
	if err := Convert(strings.NewReader(in), &out, Opts{Stderr: &errBuf}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	parsed, err := tally.Parse(strings.NewReader(out.String()), nil)
	if err != nil {
		t.Fatalf("round-trip Parse: %v", err)
	}
	if len(parsed.Rows) != 1 || parsed.Rows[0].Label != "kept" {
		t.Errorf("rows = %+v", parsed.Rows)
	}
	got := errBuf.String()
	if !strings.Contains(got, "wrap leaderboard: dropped 1") {
		t.Errorf("stderr missing oversize warning: %q", got)
	}
}

// fo-n25.6: a nil Opts.Stderr silences the oversize warning instead of
// falling back to the process's real stderr.
func TestConvert_OversizeLineNilStderrSilent(t *testing.T) {
	huge := strings.Repeat("Z", lineread.MaxLineLen+1024)
	in := huge + "\n5 kept\n"

	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out, Opts{}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
}

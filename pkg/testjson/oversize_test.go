package testjson

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/lineread"
)

// fo-gn0 regression: a single >MaxLineLen line must be counted as malformed
// and skipped, but subsequent valid events must still be parsed.

func TestParseStream_OversizeLineDoesNotAbort(t *testing.T) {
	huge := strings.Repeat("X", lineread.MaxLineLen+1024)
	input := `{"Action":"pass","Package":"a","Test":"TestA"}` + "\n" +
		huge + "\n" +
		`{"Action":"pass","Package":"b","Test":"TestB"}` + "\n"

	results, malformed, err := ParseBytes([]byte(input))
	if err != nil {
		t.Fatalf("ParseBytes err = %v", err)
	}
	if malformed != 1 {
		t.Errorf("malformed = %d, want 1", malformed)
	}
	if len(results) != 2 {
		t.Errorf("got %d packages, want 2 (a + b)", len(results))
	}
}

func TestStream_OversizeLineDoesNotAbort(t *testing.T) {
	huge := strings.Repeat("Y", lineread.MaxLineLen+1024)
	input := `{"Action":"pass","Package":"a","Test":"TestA"}` + "\n" +
		huge + "\n" +
		`{"Action":"pass","Package":"b","Test":"TestB"}` + "\n"

	var got []string
	malformed, err := Stream(context.Background(), bytes.NewReader([]byte(input)), func(ev TestEvent) {
		got = append(got, ev.Package+":"+ev.Action)
	})
	if err != nil {
		t.Fatalf("Stream err = %v", err)
	}
	if malformed != 1 {
		t.Errorf("malformed = %d, want 1", malformed)
	}
	if len(got) != 2 {
		t.Errorf("got %v events, want 2", got)
	}
}

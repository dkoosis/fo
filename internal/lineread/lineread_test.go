package lineread

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRead_Simple(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("alpha\nbeta\n"))
	for _, want := range []string{"alpha", "beta"} {
		line, ov, err := Read(br)
		if err != nil || ov || string(line) != want {
			t.Fatalf("want %q, got line=%q ov=%v err=%v", want, line, ov, err)
		}
	}
	_, _, err := Read(br)
	if !errors.Is(err, io.EOF) {
		t.Errorf("third read err = %v, want EOF", err)
	}
}

func TestRead_NoTrailingNewline(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("abc\ndef"))
	line, ov, err := Read(br)
	if err != nil || ov || string(line) != "abc" {
		t.Fatalf("first: line=%q ov=%v err=%v", line, ov, err)
	}
	line, ov, err = Read(br)
	if ov || string(line) != "def" {
		t.Fatalf("second: line=%q ov=%v err=%v", line, ov, err)
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestRead_OversizeLineSkippedAndDrained(t *testing.T) {
	huge := strings.Repeat("X", MaxLineLen+1024)
	input := "first\n" + huge + "\nthird\n"
	br := bufio.NewReader(strings.NewReader(input))

	line, ov, err := Read(br)
	if err != nil || ov || string(line) != "first" {
		t.Fatalf("first: line=%q ov=%v err=%v", line, ov, err)
	}
	_, ov, err = Read(br)
	if err != nil {
		t.Fatalf("oversize read err = %v", err)
	}
	if !ov {
		t.Errorf("expected oversize=true")
	}
	line, ov, err = Read(br)
	if err != nil || ov || string(line) != "third" {
		t.Fatalf("third: line=%q ov=%v err=%v", line, ov, err)
	}
}

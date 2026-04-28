package boundread

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAll_UnderCap(t *testing.T) {
	got, err := All(strings.NewReader("hello"), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestAll_AtCap(t *testing.T) {
	in := strings.Repeat("x", 100)
	got, err := All(strings.NewReader(in), 100)
	if err != nil {
		t.Fatalf("unexpected error at exact cap: %v", err)
	}
	if len(got) != 100 {
		t.Fatalf("got %d bytes, want 100", len(got))
	}
}

func TestAll_OverCap(t *testing.T) {
	in := strings.Repeat("x", 101)
	_, err := All(strings.NewReader(in), 100)
	if !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("got %v, want ErrInputTooLarge", err)
	}
}

func TestAll_ZeroMaxUsesDefault(t *testing.T) {
	got, err := All(strings.NewReader("hi"), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

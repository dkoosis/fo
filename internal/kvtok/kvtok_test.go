package kvtok

import (
	"errors"
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single bareword", "foo", []string{"foo"}},
		{"two barewords", "foo bar", []string{"foo", "bar"}},
		{"key=value", "k=v", []string{"k=v"}},
		{"quoted value with space", `k="a b"`, []string{"k=a b"}},
		{"quoted value with escape", `k="he said \"hi\""`, []string{`k=he said "hi"`}},
		{"backslash escape", `k="a\\b"`, []string{`k=a\b`}},
		{"tab separates", "a\tb", []string{"a", "b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Tokenize(c.in)
			if err != nil {
				t.Fatalf("Tokenize: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestTokenize_Errors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"unclosed quote", `k="open`, ErrUnclosedQuote},
		{"stray quote bareword", `foo"`, ErrStrayQuote},
		{"stray leading quote", `"x"`, ErrStrayQuote},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Tokenize(c.in)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want Is %v", err, c.want)
			}
		})
	}
}

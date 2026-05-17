// Package kvtok is a shared whitespace tokenizer for fo's tiny key=value
// header DSLs (pkg/scene and pkg/suppress). Both grammars accept
// whitespace-separated tokens where the value portion of a key=value
// pair may be a double-quoted string with backslash escapes.
//
// Extraction note: pkg/scene/scene.go (tokenizeAttrs) and
// pkg/suppress/suppress.go (tokenize) used to carry near-identical
// state machines that were already drifting on stray-quote messages.
// kvtok consolidates them so escape semantics stay in lockstep (fo-009).
package kvtok

import (
	"errors"
	"strings"
)

// ErrUnclosedQuote is returned when input ends while still inside a
// quoted region. Callers may wrap it for their own error category.
var ErrUnclosedQuote = errors.New("kvtok: unclosed quoted value")

// ErrStrayQuote is returned for a `"` that does not follow `=`.
// Bare quotes inside a key or bareword are an error, not data.
var ErrStrayQuote = errors.New("kvtok: stray quote outside key=value")

// Tokenize splits line into whitespace-separated tokens, honoring
// double-quoted values (which may contain spaces and `\"`/`\\`
// escapes). Surrounding quotes are stripped from returned tokens.
// Tokens are bareword or key=value; an isolated `"` is an error.
func Tokenize(line string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	st := state{}
	for i := range len(line) {
		if err := st.step(line[i], &cur, &toks); err != nil {
			return nil, err
		}
	}
	if st.inQuote {
		return nil, ErrUnclosedQuote
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks, nil
}

type state struct {
	inQuote bool
	escape  bool
}

func (st *state) step(c byte, cur *strings.Builder, toks *[]string) error {
	if st.escape {
		cur.WriteByte(c)
		st.escape = false
		return nil
	}
	if st.inQuote {
		switch c {
		case '\\':
			st.escape = true
		case '"':
			st.inQuote = false
		default:
			cur.WriteByte(c)
		}
		return nil
	}
	switch c {
	case ' ', '\t':
		if cur.Len() > 0 {
			*toks = append(*toks, cur.String())
			cur.Reset()
		}
	case '"':
		s := cur.String()
		if len(s) == 0 || s[len(s)-1] != '=' {
			return ErrStrayQuote
		}
		st.inQuote = true
	default:
		cur.WriteByte(c)
	}
	return nil
}

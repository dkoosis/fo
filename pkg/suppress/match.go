package suppress

import "strings"

// Ruleset is an ordered list of suppressions loaded from .fo/ignore.
// The zero value is empty and matches nothing.
type Ruleset struct {
	Rules []Suppression
}

// NewRuleset wraps parsed suppressions into a Ruleset.
func NewRuleset(rs []Suppression) *Ruleset {
	return &Ruleset{Rules: rs}
}

// Match returns the index of the first suppression in rs that matches
// (ruleID, path), or -1 if none match.
func (rs *Ruleset) Match(ruleID, path string) int {
	if rs == nil {
		return -1
	}
	for i := range rs.Rules {
		if matchSuppression(rs.Rules[i], ruleID, path) {
			return i
		}
	}
	return -1
}

// Matches reports whether s applies to (ruleID, path), ignoring expiry.
func (s Suppression) Matches(ruleID, path string) bool {
	return matchSuppression(s, ruleID, path)
}

func matchSuppression(s Suppression, ruleID, path string) bool {
	if s.RuleID != ruleID {
		return false
	}
	pat := s.Glob
	if pat == "" {
		pat = DefaultGlob
	}
	return matchGlob(pat, path)
}

// matchGlob implements a minimal doublestar-style matcher:
//   - "**" matches any sequence including path separators (and the empty string).
//   - "*"  matches any sequence not containing "/".
//   - "?"  matches any single non-"/" rune.
//
// All other runes are literal. The matcher is anchored: the pattern must
// match the entire path.
func matchGlob(pattern, name string) bool {
	return globHere(pattern, name)
}

func globHere(p, s string) bool {
	for i := range len(p) {
		c := p[i]
		switch c {
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				return globDoubleStar(p[i+2:], s)
			}
			return globStar(p[i+1:], s)
		case '?':
			if len(s) == 0 || s[0] == '/' {
				return false
			}
			s = s[1:]
		default:
			if len(s) == 0 || s[0] != c {
				return false
			}
			s = s[1:]
		}
	}
	return len(s) == 0
}

// globDoubleStar matches "**" followed by rest against s. "**" consumes
// any sequence including path separators (and the empty string).
func globDoubleStar(rest, s string) bool {
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return true
	}
	for k := 0; k <= len(s); k++ {
		if globHere(rest, s[k:]) {
			return true
		}
	}
	return false
}

// globStar matches "*" followed by rest against s. "*" consumes any
// sequence not crossing a "/" separator.
func globStar(rest, s string) bool {
	for k := 0; k <= len(s); k++ {
		if k > 0 && s[k-1] == '/' {
			return false
		}
		if globHere(rest, s[k:]) {
			return true
		}
	}
	return false
}

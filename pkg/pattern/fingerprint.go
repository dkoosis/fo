package pattern

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Fingerprint computes a stable per-finding identity =
// sha256(rule_id + "\x00" + file + "\x00" + normalize(message)).
//
// The fingerprint deliberately excludes line/column numbers so that
// unrelated edits above a defect do not invalidate its identity.
// Used by the diff classifier to match findings between fo runs.
//
// Inputs are joined with NUL separators to avoid accidental collisions
// across the (ruleID, file, message) tuple.
func Fingerprint(ruleID, file, message string) string {
	h := sha256.New()
	h.Write([]byte(ruleID))
	h.Write([]byte{0})
	h.Write([]byte(file))
	h.Write([]byte{0})
	h.Write([]byte(NormalizeMessage(message)))
	return hex.EncodeToString(h.Sum(nil))
}

// trailingCoordRe matches trailing ":line", ":line:col", or "(line, col)"
// style location suffixes that some tools embed in the message text.
var trailingCoordRe = regexp.MustCompile(`(?:\s*[:@(]\s*\d+(?:\s*[:,]\s*\d+)?\s*\)?)+\s*$`)

// absPathRe matches absolute POSIX paths or Windows drive paths embedded in messages.
// Replaced with the basename so the same defect from different checkout dirs
// (CI vs laptop, /tmp/build/x vs /home/user/x) hashes the same.
var absPathRe = regexp.MustCompile(`(?:/|[A-Za-z]:\\)[^\s:]+`)

// NormalizeMessage strips volatile elements from a finding's message so that
// the same defect surviving a line shift, path move, or working-directory
// change still produces the same fingerprint.
//
// Normalizations applied:
//   - Trim whitespace.
//   - Replace absolute paths with their basename.
//   - Strip trailing ":line", ":line:col", "(line, col)" coordinate suffixes.
//   - Collapse runs of whitespace to a single space.
func NormalizeMessage(msg string) string {
	s := strings.TrimSpace(msg)

	// Replace absolute paths with their basename.
	s = absPathRe.ReplaceAllStringFunc(s, func(p string) string {
		// Trim trailing punctuation that the regex may have grabbed.
		trail := ""
		for len(p) > 0 {
			last := p[len(p)-1]
			if last == '.' || last == ',' || last == ';' || last == ')' {
				trail = string(last) + trail
				p = p[:len(p)-1]
				continue
			}
			break
		}
		// Use forward-slash basename for both POSIX and Windows.
		idx := strings.LastIndexAny(p, `/\`)
		if idx >= 0 && idx < len(p)-1 {
			return p[idx+1:] + trail
		}
		return p + trail
	})

	// Strip trailing coord suffixes — apply repeatedly so ":12:3" then ":4" both go.
	for {
		stripped := trailingCoordRe.ReplaceAllString(s, "")
		if stripped == s {
			break
		}
		s = strings.TrimSpace(stripped)
	}

	// Collapse whitespace.
	s = strings.Join(strings.Fields(s), " ")
	return s
}

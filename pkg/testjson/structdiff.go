package testjson

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
)

// detectStructuralDiff inspects a failing test's raw output for a
// recognizable structural-diff shape — go-cmp's indented struct/slice/map
// report, or cupaloy's difflib-style unified diff — and decomposes it into
// field-level changes. Returns nil when output matches neither shape;
// callers keep the raw Output and pkg/view falls back to its existing
// line-oriented rendering.
//
// Only these two shapes are recognized (fo-d84): go-cmp is fo's own test
// style and the common case in Go generally; cupaloy is the other library
// named by the bead. Anything else — plain assertion text, testify output,
// etc. — is intentionally left alone rather than guessed at.
func detectStructuralDiff(output string) *report.StructuralDiff {
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	if isUnifiedDiff(lines) {
		if fields := parseUnifiedDiff(lines); len(fields) > 0 {
			return &report.StructuralDiff{Kind: "cupaloy", Fields: fields}
		}
		return nil
	}
	if fields := parseGoCmpDiff(lines); len(fields) > 0 {
		return &report.StructuralDiff{Kind: "go-cmp", Fields: fields}
	}
	return nil
}

// isUnifiedDiff reports whether lines carry a difflib-style unified diff
// header — cupaloy's shape: "--- ", "+++ ", and at least one "@@" hunk
// marker. go-cmp's report never emits any of the three.
func isUnifiedDiff(lines []string) bool {
	var sawOld, sawNew, sawHunk bool
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "--- "):
			sawOld = true
		case strings.HasPrefix(l, "+++ "):
			sawNew = true
		case strings.HasPrefix(l, "@@"):
			sawHunk = true
		}
	}
	return sawOld && sawNew && sawHunk
}

// looksLikeGoCmpShape requires at least one removed and one added line —
// same "both directions" guard pkg/view/diffout.go uses for its line-level
// classification, duplicated here rather than imported: testjson and view
// are siblings over report, and this is five lines.
func looksLikeGoCmpShape(lines []string) bool {
	var del, add bool
	for _, l := range lines {
		if l == "" {
			continue
		}
		switch l[0] {
		case '-':
			del = true
		case '+':
			add = true
		}
		if del && add {
			return true
		}
	}
	return false
}

// fieldKeyRe extracts a "key" from a changed diff line's content (marker
// already stripped), matching JSON ("key": value), Go-identifier
// (key: value), and spew-dump (Key: (type) value) shapes. The value group
// is non-greedy with an optional trailing comma.
var fieldKeyRe = regexp.MustCompile(`^\s*"?([A-Za-z_][\w.]*)"?:\s*(?:\([^)]*\)\s*)?(.*?),?\s*$`)

// splitKeyValue extracts (key, value) from diff-line content. ok is false
// when no key:value pattern matched; value is then the whole trimmed
// content and callers fall back to using it directly.
func splitKeyValue(content string) (key, value string, ok bool) {
	m := fieldKeyRe.FindStringSubmatch(content)
	if m == nil {
		return "", strings.TrimSpace(content), false
	}
	return m[1], m[2], true
}

// fallbackPath names a field with no recoverable key: 1-based position
// among the fields decomposed so far, so the reader still gets an anchor
// ("line 3") instead of nothing.
func fallbackPath(key string, idx int) string {
	if key != "" {
		return key
	}
	return "line " + strconv.Itoa(idx+1)
}

// parseUnifiedDiff decomposes a difflib-style unified diff (cupaloy's
// shape) into field-level changes. Removed/added lines pair positionally
// within one hunk — difflib emits a same-count removed-then-added block
// for a pure value change. A hunk boundary or an unpaired context line
// flushes any pending removed lines as removal-only fields so unrelated
// hunks never cross-pair.
func parseUnifiedDiff(lines []string) []report.DiffField {
	var fields []report.DiffField
	var pendingRemoved []string
	flush := func() {
		for _, r := range pendingRemoved {
			key, val, _ := splitKeyValue(r)
			fields = append(fields, report.DiffField{Path: fallbackPath(key, len(fields)), Removed: val})
		}
		pendingRemoved = pendingRemoved[:0]
	}
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "--- "), strings.HasPrefix(l, "+++ "):
			continue
		case strings.HasPrefix(l, "@@"):
			flush()
		case strings.HasPrefix(l, "-"):
			pendingRemoved = append(pendingRemoved, l[1:])
		case strings.HasPrefix(l, "+"):
			added := l[1:]
			addKey, addVal, _ := splitKeyValue(added)
			if len(pendingRemoved) > 0 {
				rem := pendingRemoved[0]
				pendingRemoved = pendingRemoved[1:]
				remKey, remVal, _ := splitKeyValue(rem)
				path := addKey
				if path == "" {
					path = remKey
				}
				fields = append(fields, report.DiffField{Path: fallbackPath(path, len(fields)), Removed: remVal, Added: addVal})
				continue
			}
			fields = append(fields, report.DiffField{Path: fallbackPath(addKey, len(fields)), Added: addVal})
		default:
			flush() // context line — end the current pairing window
		}
	}
	flush()
	return fields
}

// parseGoCmpDiff decomposes go-cmp's indented struct/slice/map diff report
// into field-level changes using a path stack keyed by nesting: a context
// line ending in "{"/"[" pushes a path segment (the field name if the line
// has one, else the bare type name); a lone "}"/"]" pops it. A "-" line
// followed by a "+" line pairs into one DiffField; an unpaired "-" or "+"
// records a removal- or addition-only field.
func parseGoCmpDiff(lines []string) []report.DiffField {
	if !looksLikeGoCmpShape(lines) {
		return nil
	}
	p := &goCmpParser{}
	for _, l := range lines {
		p.step(l)
	}
	p.flushDel()
	return p.fields
}

// goCmpParser walks a go-cmp report line by line, holding the nesting
// stack and any not-yet-paired "-" line. Split out of parseGoCmpDiff to
// keep each step's branching small.
type goCmpParser struct {
	fields     []report.DiffField
	stack      []string
	pendingDel *string
}

// step processes one line of the report.
func (p *goCmpParser) step(l string) {
	if l == "" {
		return
	}
	marker := l[0]
	content := strings.TrimLeft(strings.TrimPrefix(l[1:], " "), "\t")
	trimmed := strings.TrimRight(content, ",")

	switch {
	case strings.HasSuffix(trimmed, "{"), strings.HasSuffix(trimmed, "["):
		p.flushDel()
		p.stack = append(p.stack, structName(trimmed))
	case trimmed == "}" || trimmed == "]":
		p.flushDel()
		if len(p.stack) > 0 {
			p.stack = p.stack[:len(p.stack)-1]
		}
	case marker == '-':
		p.flushDel()
		c := content
		p.pendingDel = &c
	case marker == '+':
		p.handlePlus(content)
	default:
		p.flushDel() // unchanged context line inside the struct body
	}
}

// handlePlus resolves a "+" line: paired with a pending "-" it closes one
// DiffField; unpaired it's an addition-only field.
func (p *goCmpParser) handlePlus(content string) {
	addKey, addVal, _ := splitKeyValue(content)
	if p.pendingDel == nil {
		p.fields = append(p.fields, report.DiffField{Path: joinPath(p.stack, addKey), Added: addVal})
		return
	}
	delKey, delVal, _ := splitKeyValue(*p.pendingDel)
	key := addKey
	if key == "" {
		key = delKey
	}
	p.fields = append(p.fields, report.DiffField{Path: joinPath(p.stack, key), Removed: delVal, Added: addVal})
	p.pendingDel = nil
}

// flushDel resolves a pending "-" line with no matching "+" as a
// removal-only field.
func (p *goCmpParser) flushDel() {
	if p.pendingDel == nil {
		return
	}
	key, val, _ := splitKeyValue(*p.pendingDel)
	p.fields = append(p.fields, report.DiffField{Path: joinPath(p.stack, key), Removed: val})
	p.pendingDel = nil
}

// structName labels a pushed path segment from an opening line like
// "Sub: root.Sub{" (→ "Sub", the field name) or a bare "MyStruct{" at the
// diff's root (→ "MyStruct", the type name — no field name to prefer).
func structName(opening string) string {
	if key, _, ok := splitKeyValue(opening); ok {
		return key
	}
	return strings.TrimSuffix(strings.TrimSuffix(opening, "{"), "[")
}

// joinPath dot-joins the current nesting stack and a leaf field name.
// Falls back to "value" when both are empty (e.g. a scalar-only diff with
// no struct wrapper at all).
func joinPath(stack []string, key string) string {
	parts := make([]string, 0, len(stack)+1)
	parts = append(parts, stack...)
	if key != "" {
		parts = append(parts, key)
	}
	if len(parts) == 0 {
		return "value"
	}
	return strings.Join(parts, ".")
}

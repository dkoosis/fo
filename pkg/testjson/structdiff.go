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
		// Header looked coherent but didn't yield fields (e.g. a hunk shape
		// this parser doesn't recognize) — don't commit to "not a diff";
		// give go-cmp detection a chance below rather than returning nil.
	}
	if fields := parseGoCmpDiff(lines); len(fields) > 0 {
		return &report.StructuralDiff{Kind: "go-cmp", Fields: fields}
	}
	return nil
}

// isUnifiedDiff reports whether lines carry a coherent difflib-style
// unified diff header — cupaloy's shape: a "--- " line immediately
// followed by a "+++ " line, then at least one "@@" hunk marker later in
// the output. Requiring the two header lines adjacent (not merely present
// somewhere in the output) keeps ordinary output that happens to contain
// a "--- " line and, unrelated, a later "+++ " line and an "@@" line from
// being misread as a unified diff. go-cmp's report never emits any of the
// three.
func isUnifiedDiff(lines []string) bool {
	for i := 0; i+1 < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "--- ") || !strings.HasPrefix(lines[i+1], "+++ ") {
			continue
		}
		for _, l := range lines[i+2:] {
			if strings.HasPrefix(l, "@@") {
				return true
			}
		}
	}
	return false
}

// looksLikeGoCmpShape requires an actual paired removal-then-addition —
// a "-"-marked line immediately followed (blank lines aside) by a
// "+"-marked line, the shape go-cmp's reporter emits for a changed value.
// A weaker "a '-' exists and a '+' exists somewhere in the output" check
// misfires on ordinary output that merely contains two unrelated lines
// starting with those characters.
func looksLikeGoCmpShape(lines []string) bool {
	var prevDel bool
	for _, l := range lines {
		if l == "" {
			continue
		}
		switch l[0] {
		case '-':
			prevDel = true
			continue
		case '+':
			if prevDel {
				return true
			}
		}
		prevDel = false
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
	p.closeBlock() // resolve any still-open block (malformed/truncated input)
	p.flushPendingBlk()
	p.flushDel()
	return p.fields
}

// goCmpParser walks a go-cmp report line by line, holding the nesting
// stack, any not-yet-paired "-" line, and any in-progress marked block
// (see blockDir). Split out of parseGoCmpDiff to keep each step's
// branching small.
type goCmpParser struct {
	fields     []report.DiffField
	stack      []string
	pendingDel *string

	// blockDir, blockDepth and blockLines capture a marked line that opens
	// a brace/bracket — e.g. "-\tSub: root.Sub{" — and everything nested
	// inside it up to the matching marked close. go-cmp emits this shape
	// when it replaces a nested struct/slice wholesale rather than diffing
	// it field by field: both the open line and every line inside carry
	// the SAME marker. Treating each such line as ordinary content (the
	// old bug: a marked close-brace line forced any pending "-" to flush
	// as removal-only before the mirrored "+" block ever arrived) split
	// one wholesale change into two unpaired fields at the same path.
	// Capturing the whole marked block and pairing it with its "+"/"-"
	// mirror keeps it one field.
	blockDir   byte
	blockDepth int
	blockLines []string
	pendingBlk *blockValue
}

// blockValue holds one resolved side (removed or added) of a captured
// marked block, keyed by its path, awaiting the mirrored side.
type blockValue struct {
	path string
	val  string
}

// step processes one line of the report.
func (p *goCmpParser) step(l string) {
	if l == "" {
		return
	}
	marker := l[0]
	content := strings.TrimLeft(strings.TrimPrefix(l[1:], " "), "\t")
	trimmed := strings.TrimRight(content, ",")

	if p.blockDir != 0 {
		if marker == p.blockDir {
			p.extendBlock(content, trimmed)
			return
		}
		// Marker changed mid-block: malformed/unexpected input. Close
		// what was captured (best effort) and handle this line normally.
		p.closeBlock()
	}
	p.handleLine(marker, content, trimmed)
}

// handleLine dispatches one non-block-continuation line: a marked line
// opening a brace/bracket starts a block capture (see startBlock); an
// unmarked one is ordinary stack bookkeeping; otherwise it's a plain
// removed/added leaf line.
func (p *goCmpParser) handleLine(marker byte, content, trimmed string) {
	switch {
	case (marker == '-' || marker == '+') && (strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, "[")):
		p.flushDel()
		p.startBlock(marker, content)
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

// startBlock begins capturing a marked brace/bracket line and everything
// nested inside it as one opaque value.
func (p *goCmpParser) startBlock(marker byte, openLine string) {
	p.blockDir = marker
	p.blockDepth = 1
	p.blockLines = []string{openLine}
}

// extendBlock consumes one line while a marked block is open, tracking
// brace/bracket depth so a nested struct inside the block doesn't close
// it early. Depth reaching zero resolves the block.
func (p *goCmpParser) extendBlock(content, trimmed string) {
	p.blockLines = append(p.blockLines, content)
	switch {
	case strings.HasSuffix(trimmed, "{"), strings.HasSuffix(trimmed, "["):
		p.blockDepth++
	case trimmed == "}" || trimmed == "]":
		p.blockDepth--
		if p.blockDepth == 0 {
			p.closeBlock()
		}
	}
}

// closeBlock resolves a completed (or malformed/truncated) marked block
// into a DiffField: a "-" block is held as pendingBlk awaiting its "+"
// mirror; a "+" block pairs with a pending "-" block at the same path, or
// stands alone as an addition-only field.
func (p *goCmpParser) closeBlock() {
	if p.blockDir == 0 {
		return
	}
	key, firstVal, ok := splitKeyValue(p.blockLines[0])
	if !ok {
		firstVal = p.blockLines[0]
	}
	parts := append([]string{firstVal}, p.blockLines[1:]...)
	val := strings.TrimSuffix(strings.Join(parts, " "), ",")
	path := joinPath(p.stack, key)
	dir := p.blockDir
	p.blockDir = 0
	p.blockDepth = 0
	p.blockLines = nil

	if dir == '-' {
		p.flushPendingBlk()
		p.pendingBlk = &blockValue{path: path, val: val}
		return
	}
	if p.pendingBlk != nil && p.pendingBlk.path == path {
		p.fields = append(p.fields, report.DiffField{Path: path, Removed: p.pendingBlk.val, Added: val})
		p.pendingBlk = nil
		return
	}
	p.flushPendingBlk()
	p.fields = append(p.fields, report.DiffField{Path: path, Added: val})
}

// flushPendingBlk resolves a pending "-" block with no matching "+" block
// as a removal-only field.
func (p *goCmpParser) flushPendingBlk() {
	if p.pendingBlk == nil {
		return
	}
	p.fields = append(p.fields, report.DiffField{Path: p.pendingBlk.path, Removed: p.pendingBlk.val})
	p.pendingBlk = nil
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

// Package scene parses fo's scene input format — narrated, multi-actor
// walk-throughs grouped into numbered acts. Used by tools like loto's
// `make demo` to ship a polished transcript as the primary deliverable
// rather than `go test -v` framing.
//
// Format:
//
//	# fo:scene [title=<string>] [actors=<csv>]
//	## <act-number> · <act-title>
//	> <narration line>
//	@<actor> $ <command>
//	  <output line>
//	  (exit <N>)
//
// Rules:
//   - `>` lines are narration beats; preserved text is what follows `> `.
//   - `@actor $ cmd` opens a command beat. Subsequent lines indented by
//     exactly two spaces are captured output. The first non-indented or
//     differently-shaped line ends the command. An `(exit N)` line, if
//     present as the last indented line, sets Command.Exit; default 0.
//   - Blank lines inside an act separate beats. Blank lines before the
//     first `##` are ignored.
//   - `#` comment lines after the header (not the header itself) are
//     ignored.
//
// Grammar note: an output line that itself looks like `  (exit N)` is
// treated as the exit trailer iff it is the final indented line of the
// command's run. Other parenthesized indented lines are kept verbatim
// in Output. Lines with non-2-space indentation (tab, single space,
// 3+ spaces) terminate the command.
package scene

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const HeaderPrefix = "# fo:scene"

// BeatKind discriminates the Beat union.
type BeatKind int

const (
	BeatNarration BeatKind = iota
	BeatCommand
)

// Beat is one narrative or command unit inside an Act.
type Beat struct {
	Kind      BeatKind `json:"kind"`
	Narration string   `json:"narration,omitempty"`
	Command   Command  `json:"command,omitzero"`
}

// Command is an invocation beat: `@actor $ cmd` plus output and exit.
type Command struct {
	Actor  string   `json:"actor"`
	Cmd    string   `json:"cmd"`
	Output []string `json:"output,omitempty"`
	Exit   int      `json:"exit"`
}

// Act groups beats under a numbered title.
type Act struct {
	Number string `json:"number"`
	Title  string `json:"title"`
	Beats  []Beat `json:"beats,omitempty"`
}

// Scene is a parsed `# fo:scene` document.
type Scene struct {
	Title  string   `json:"title,omitempty"`
	Actors []string `json:"actors,omitempty"`
	Acts   []Act    `json:"acts,omitempty"`
}

// IsHeader reports whether data starts (modulo leading whitespace and
// blank lines) with the scene header prefix.
func IsHeader(data []byte) bool {
	headerPrefix := []byte(HeaderPrefix)
	s := data
	for {
		nl := bytes.IndexByte(s, '\n')
		var line []byte
		if nl < 0 {
			line = s
			s = nil
		} else {
			line = s[:nl]
			s = s[nl+1:]
		}
		trimmed := bytes.TrimLeft(line, " \t\r")
		if len(trimmed) == 0 {
			if nl < 0 {
				return false
			}
			continue
		}
		return bytes.HasPrefix(trimmed, headerPrefix)
	}
}

var (
	errNoHeader       = errors.New("scene: missing '# fo:scene' header")
	errMalformedAct   = errors.New("scene: malformed act header")
	errMalformedActor = errors.New("scene: malformed actor line")
	errMalformedExit  = errors.New("scene: malformed exit trailer")
	errUnknownAttr    = errors.New("scene: unknown header attr")
)

// Parse reads a scene document from r.
func Parse(r io.Reader) (Scene, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)

	p := &parser{}
	for sc.Scan() {
		p.lineNo++
		if err := p.feed(sc.Text()); err != nil {
			return Scene{}, err
		}
	}
	if err := sc.Err(); err != nil {
		return Scene{}, fmt.Errorf("scene: read: %w", err)
	}
	p.flushCmd()
	if !p.headerSeen {
		return Scene{}, errNoHeader
	}
	return p.s, nil
}

type parser struct {
	s          Scene
	headerSeen bool
	lineNo     int
	curAct     *Act
	curCmd     *Command
}

func (p *parser) flushCmd() {
	if p.curCmd == nil {
		return
	}
	// The exit trailer is only a trailer if it's the FINAL indented
	// line of the command block. Detect it here at flush time so an
	// `(exit N)` appearing mid-output is treated as literal output.
	if n := len(p.curCmd.Output); n > 0 {
		if exit, ok, err := parseExitTrailer(p.curCmd.Output[n-1]); err == nil && ok {
			p.curCmd.Exit = exit
			p.curCmd.Output = p.curCmd.Output[:n-1]
		}
	}
	p.curAct.Beats = append(p.curAct.Beats, Beat{Kind: BeatCommand, Command: *p.curCmd})
	p.curCmd = nil
}

func (p *parser) feed(raw string) error {
	if !p.headerSeen {
		return p.feedHeader(raw)
	}
	if p.curCmd != nil && isOutputLine(raw) {
		return p.feedOutput(raw[2:])
	}
	p.flushCmd()
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return p.feedBody(trimmed)
}

func (p *parser) feedHeader(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(trimmed, HeaderPrefix) {
		return errNoHeader
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, HeaderPrefix))
	if err := parseHeaderAttrs(rest, &p.s); err != nil {
		return fmt.Errorf("scene: line %d: %w", p.lineNo, err)
	}
	p.headerSeen = true
	return nil
}

func (p *parser) feedOutput(body string) error {
	exit, ok, err := parseExitTrailer(body)
	if err != nil {
		return fmt.Errorf("scene: line %d: %w", p.lineNo, err)
	}
	if ok {
		p.curCmd.Exit = exit
		p.flushCmd()
		return nil
	}
	p.curCmd.Output = append(p.curCmd.Output, body)
	return nil
}

func (p *parser) feedBody(trimmed string) error {
	switch {
	case strings.HasPrefix(trimmed, "## "):
		act, err := parseActHeader(trimmed)
		if err != nil {
			return fmt.Errorf("scene: line %d: %w", p.lineNo, err)
		}
		p.s.Acts = append(p.s.Acts, act)
		p.curAct = &p.s.Acts[len(p.s.Acts)-1]
		return nil
	case strings.HasPrefix(trimmed, "#"):
		return nil
	case strings.HasPrefix(trimmed, "> ") || trimmed == ">":
		if p.curAct == nil {
			return fmt.Errorf("scene: line %d: %w: narration before any act", p.lineNo, errMalformedAct)
		}
		text := strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " ")
		p.curAct.Beats = append(p.curAct.Beats, Beat{Kind: BeatNarration, Narration: text})
		return nil
	case strings.HasPrefix(trimmed, "@"):
		if p.curAct == nil {
			return fmt.Errorf("scene: line %d: %w: command before any act", p.lineNo, errMalformedActor)
		}
		cmd, err := parseActorLine(trimmed)
		if err != nil {
			return fmt.Errorf("scene: line %d: %w", p.lineNo, err)
		}
		p.curCmd = &cmd
		return nil
	}
	return fmt.Errorf("scene: line %d: %w: %q", p.lineNo, errMalformedAct, trimmed)
}

func isOutputLine(raw string) bool {
	if len(raw) < 2 {
		return false
	}
	if raw[0] != ' ' || raw[1] != ' ' {
		return false
	}
	// reject 3+ leading spaces or tab indent (terminates command).
	if len(raw) >= 3 && (raw[2] == ' ' || raw[2] == '\t') {
		return false
	}
	return true
}

func parseExitTrailer(body string) (int, bool, error) {
	t := strings.TrimSpace(body)
	if !strings.HasPrefix(t, "(exit") || !strings.HasSuffix(t, ")") {
		return 0, false, nil
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(t, "(exit"), ")")
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return 0, false, fmt.Errorf("%w: missing exit code in %q", errMalformedExit, body)
	}
	n, err := strconv.Atoi(inner)
	if err != nil {
		return 0, false, fmt.Errorf("%w: %q", errMalformedExit, inner)
	}
	return n, true, nil
}

func parseActHeader(line string) (Act, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "##"))
	// expect: N · title (middle-dot separator)
	numTok, titleTok, ok := strings.Cut(rest, "·")
	if !ok {
		return Act{}, fmt.Errorf("%w: expected 'N · title', got %q", errMalformedAct, line)
	}
	number := strings.TrimSpace(numTok)
	title := strings.TrimSpace(titleTok)
	if number == "" || title == "" {
		return Act{}, fmt.Errorf("%w: empty number or title in %q", errMalformedAct, line)
	}
	return Act{Number: number, Title: title}, nil
}

func parseActorLine(line string) (Command, error) {
	// `@actor $ cmd` — actor has no whitespace; `$` separates command.
	rest := strings.TrimPrefix(line, "@")
	sp := strings.IndexAny(rest, " \t")
	if sp <= 0 {
		return Command{}, fmt.Errorf("%w: expected '@actor $ cmd', got %q", errMalformedActor, line)
	}
	actor := rest[:sp]
	tail := strings.TrimLeft(rest[sp:], " \t")
	if !strings.HasPrefix(tail, "$") {
		return Command{}, fmt.Errorf("%w: missing '$' after actor in %q", errMalformedActor, line)
	}
	cmd := strings.TrimLeft(strings.TrimPrefix(tail, "$"), " \t")
	if cmd == "" {
		return Command{}, fmt.Errorf("%w: empty command in %q", errMalformedActor, line)
	}
	return Command{Actor: actor, Cmd: cmd}, nil
}

func parseHeaderAttrs(tail string, s *Scene) error {
	toks, err := tokenizeAttrs(tail)
	if err != nil {
		return err
	}
	for _, tok := range toks {
		if err := applyAttr(tok, s); err != nil {
			return err
		}
	}
	return nil
}

func applyAttr(tok string, s *Scene) error {
	key, val, ok := strings.Cut(tok, "=")
	if !ok || key == "" {
		return fmt.Errorf("%w: expected key=value, got %q", errUnknownAttr, tok)
	}
	switch strings.ToLower(key) {
	case "title":
		s.Title = val
	case "actors":
		for a := range strings.SplitSeq(val, ",") {
			if t := strings.TrimSpace(a); t != "" {
				s.Actors = append(s.Actors, t)
			}
		}
	default:
		return fmt.Errorf("%w: %q", errUnknownAttr, key)
	}
	return nil
}

// tokenizeAttrs splits on whitespace, honoring double-quoted values
// after `=`. Quote-aware; mirrors pkg/suppress conventions.
func tokenizeAttrs(line string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	st := attrTokState{}
	for i := range len(line) {
		if err := st.step(line[i], &cur, &toks); err != nil {
			return nil, err
		}
	}
	if st.inQuote {
		return nil, fmt.Errorf("%w: unclosed quote in header", errUnknownAttr)
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks, nil
}

type attrTokState struct {
	inQuote bool
	escape  bool
}

func (st *attrTokState) step(c byte, cur *strings.Builder, toks *[]string) error {
	if st.escape {
		cur.WriteByte(c)
		st.escape = false
		return nil
	}
	if st.inQuote {
		return st.stepInQuote(c, cur)
	}
	return st.stepBare(c, cur, toks)
}

func (st *attrTokState) stepInQuote(c byte, cur *strings.Builder) error {
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

func (st *attrTokState) stepBare(c byte, cur *strings.Builder, toks *[]string) error {
	switch c {
	case ' ', '\t':
		if cur.Len() > 0 {
			*toks = append(*toks, cur.String())
			cur.Reset()
		}
	case '"':
		s := cur.String()
		if len(s) == 0 || s[len(s)-1] != '=' {
			return fmt.Errorf("%w: stray '\"' in header", errUnknownAttr)
		}
		st.inQuote = true
	default:
		cur.WriteByte(c)
	}
	return nil
}

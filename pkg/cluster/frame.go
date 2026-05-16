package cluster

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Stack-frame regexes. Panic stacks come in pairs:
//   <importpath>.<Func>(args)
//           /abs/path/file.go:N +0xHEX
//
// testify and t.Errorf instead surface a single line like:
//   file.go:42:
// or:
//   Error Trace: /abs/path/file.go:42

var (
	panicFrameFile = regexp.MustCompile(`^\s+(\S+\.go):(\d+)(?:\s|$)`)
	panicFuncLine  = regexp.MustCompile(`^([\w./*()-]+\.\w+)\(`)
	errorTraceLine = regexp.MustCompile(`Error Trace:\s*(\S+\.go):(\d+)`)
	simpleCite     = regexp.MustCompile(`(?m)^\s*([^\s:/]+\.go):(\d+):`)
)

// stdlibDirs identifies stdlib packages by their canonical src/<pkg>/
// directory segment. Any frame whose path contains src/<one of these>/
// is treated as runtime, not user code.
var stdlibDirs = []string{
	"runtime", "testing", "reflect", "sync", "os", "net", "io",
	"fmt", "errors", "strings", "bytes", "encoding", "time", "context",
}

var (
	stdlibPathRE  = regexp.MustCompile(`(?:^|/)src/(?:` + strings.Join(stdlibDirs, "|") + `)/`)
	testifyPathRE = regexp.MustCompile(`/stretchr/testify[^/]*/(?:assert|require|mock|suite)/`)
	runtimePathRE = regexp.MustCompile(`(?:^|/)runtime/[^/]+\.go$`)
)

func isUserCodeFile(path string) bool {
	if path == "" {
		return false
	}
	if stdlibPathRE.MatchString(path) {
		return false
	}
	if testifyPathRE.MatchString(path) {
		return false
	}
	if runtimePathRE.MatchString(path) {
		return false
	}
	return true
}

func isUserCodeFunc(fn string) bool {
	if fn == "" {
		return true
	}
	for _, p := range []string{"runtime.", "testing.", "reflect."} {
		if strings.HasPrefix(fn, p) {
			return false
		}
	}
	return true
}

// extractTopUserFrame walks failure output looking for the innermost
// frame whose file qualifies as user code. Returns "" if no qualifying
// frame is found.
//
// The returned form is path:line. The function name is intentionally
// omitted from the cluster key — a single line can host multiple
// sub-calls and line is the strongest stable signal within a run.
func extractTopUserFrame(output string, keepAbsPaths bool) string {
	if f := scanPanicStack(output, keepAbsPaths); f != "" {
		return f
	}
	if f := matchCite(errorTraceLine, output, keepAbsPaths); f != "" {
		return f
	}
	if f := matchCite(simpleCite, output, keepAbsPaths); f != "" {
		return f
	}
	return ""
}

func scanPanicStack(output string, keepAbsPaths bool) string {
	lines := strings.Split(output, "\n")
	for i := range lines {
		m := panicFrameFile.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		path, lno := m[1], m[2]
		if !isUserCodeFile(path) {
			continue
		}
		if !isUserCodeFunc(funcNameAbove(lines, i)) {
			continue
		}
		return formatFrame(path, lno, keepAbsPaths)
	}
	return ""
}

func funcNameAbove(lines []string, i int) string {
	if i <= 0 {
		return ""
	}
	fm := panicFuncLine.FindStringSubmatch(strings.TrimSpace(lines[i-1]))
	if fm == nil {
		return ""
	}
	return fm[1]
}

func matchCite(re *regexp.Regexp, output string, keepAbsPaths bool) string {
	m := re.FindStringSubmatch(output)
	if m == nil {
		return ""
	}
	path, lno := m[1], m[2]
	if !isUserCodeFile(path) {
		return ""
	}
	return formatFrame(path, lno, keepAbsPaths)
}

func formatFrame(path, line string, keepAbs bool) string {
	if !keepAbs {
		path = trimAbsPath(path)
	}
	return path + ":" + line
}

// trimAbsPath reduces /Users/x/proj/pkg/foo/bar.go to pkg/foo/bar.go
// when a recognizable Go-module-ish segment exists; otherwise falls
// back to basename.
func trimAbsPath(path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	// Look for the deepest occurrence of a known top-level segment
	// like "pkg/", "internal/", "cmd/", "src/". Cheap and good enough
	// for canonicalizing user-code paths.
	for _, anchor := range []string{"/pkg/", "/internal/", "/cmd/", "/src/"} {
		if idx := strings.LastIndex(path, anchor); idx >= 0 {
			return strings.TrimPrefix(path[idx:], "/")
		}
	}
	return filepath.Base(path)
}

package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func executeCommand(args ...string) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	code := run(args, bytes.NewReader(nil), &outBuf, &errBuf)
	if code != 0 {
		return outBuf.String(), errBuf.String(), &exitError{code: code}
	}
	return outBuf.String(), errBuf.String(), nil
}

type exitError struct{ code int }

func (e *exitError) Error() string { return "non-zero exit" }

type helpNode struct {
	name     string
	args     []string
	visible  bool
	children []helpNode
}

func walkVisibleHelp(nodes []helpNode, visit func(name string, args []string)) {
	var walk func(prefixName string, n helpNode)
	walk = func(prefixName string, n helpNode) {
		fullName := strings.TrimSpace(strings.TrimSpace(prefixName + " " + n.name))
		if n.visible {
			path := append(append([]string{}, n.args...), "--help")
			visit(fullName, path)
		}
		for _, ch := range n.children {
			walk(fullName, ch)
		}
	}
	for _, n := range nodes {
		walk("", n)
	}
}

func normalizeHelp(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	out := strings.Join(lines, "\n")
	out = strings.TrimRight(out, "\n")
	return out + "\n"
}

func readGolden(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return string(b)
}

func writeGolden(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func goldenName(name string) string {
	if name == "root" {
		return "root.golden"
	}
	return strings.ReplaceAll(name, " ", "-") + ".golden"
}

func TestHelpGolden(t *testing.T) {
	commands := []helpNode{{
		name:    "root",
		args:    nil,
		visible: true,
		children: []helpNode{{
			name:    "wrap",
			args:    []string{"wrap"},
			visible: true,
			children: []helpNode{
				{name: "archlint", args: []string{"wrap", "archlint"}, visible: true},
				{name: "diag", args: []string{"wrap", "diag"}, visible: true},
				{name: "jscpd", args: []string{"wrap", "jscpd"}, visible: true},
			},
		}},
	}}

	walkVisibleHelp(commands, func(name string, args []string) {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := executeCommand(args...)
			if err != nil {
				t.Fatalf("run %q: %v (stderr=%q)", strings.Join(args, " "), err, stderr)
			}
			got := normalizeHelp(stdout + stderr)
			goldenPath := filepath.Join("testdata", "help", goldenName(name))
			if *update {
				if err := writeGolden(goldenPath, got); err != nil {
					t.Fatalf("write golden %s: %v", goldenPath, err)
				}
				return
			}
			want := normalizeHelp(readGolden(t, goldenPath))
			if got != want {
				t.Fatalf("help output mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
			}
		})
	})
}

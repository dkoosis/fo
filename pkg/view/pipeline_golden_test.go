package view_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	foBinOnce     sync.Once
	foBinPath     string
	errFoBinBuild error
)

// foBinary builds cmd/fo once per test process and returns the path.
// Lives next to TestPipelineGoldens (rather than in TestMain) because
// view_test.go already owns TestMain.
func foBinary(t *testing.T) string {
	t.Helper()
	foBinOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			errFoBinBuild = buildError("runtime.Caller failed")
			return
		}
		// thisFile = .../pkg/view/pipeline_golden_test.go → repo = ../..
		repo := filepath.Join(filepath.Dir(thisFile), "..", "..")
		dir, err := os.MkdirTemp("", "fo-bin-*")
		if err != nil {
			errFoBinBuild = err
			return
		}
		bin := filepath.Join(dir, "fo")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/fo")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			errFoBinBuild = buildError("go build: " + err.Error() + "\n" + string(out))
			return
		}
		foBinPath = bin
	})
	if errFoBinBuild != nil {
		t.Fatalf("build fo: %v", errFoBinBuild)
	}
	return foBinPath
}

type buildError string

func (e buildError) Error() string { return string(e) }

// TestPipelineGoldens replays each captured stdin stream in
// testdata/pipelines/*.in through the built `fo` binary in two
// formats (human, llm) and diffs the output against the committed
// golden. Refresh with: go test ./pkg/view/... -update
func TestPipelineGoldens(t *testing.T) {
	bin := foBinary(t)

	matches, err := filepath.Glob("testdata/pipelines/*.in")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no pipeline fixtures found in testdata/pipelines/")
	}

	for _, in := range matches {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		for _, format := range []string{"human", "llm"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				inBytes, err := os.ReadFile(in)
				if err != nil {
					t.Fatalf("read %s: %v", in, err)
				}
				cmd := exec.Command(bin, "--format", format, "--no-state")
				cmd.Stdin = bytes.NewReader(inBytes)
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &out
				// Exit 0 (clean) and 1 (findings present) are expected;
				// any other code is a real failure.
				if err := cmd.Run(); err != nil {
					var ee *exec.ExitError
					if !errors.As(err, &ee) || ee.ExitCode() > 1 {
						t.Fatalf("fo crashed on %s: %v\n%s", in, err, out.String())
					}
				}

				goldenPath := filepath.Join("testdata", "pipelines", name+"."+format+".golden")
				got := out.Bytes()

				if *update {
					if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
						t.Fatalf("write golden: %v", err)
					}
					return
				}

				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("golden mismatch for %s:\n--- want ---\n%s\n--- got ---\n%s",
						goldenPath, want, got)
				}
			})
		}
	}
}

// Fixture-iterator e2e tests: walk testdata/golden/v2/ and exercise the full
// pipeline across every fixture. These stay in Go (rather than .txtar) because
// they dynamically discover scenarios and assert against checked-in goldens —
// patterns testscript does not express cleanly.
//
// Single-scenario e2e tests live in testdata/script/*.txtar (see TestScripts).
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fixturesRoot    = "../../testdata/golden/v2"
	needsFormatRule = "needs-formatting"
)

// scenario describes one fixture run, fully discovered from the filesystem.
type scenario struct {
	dir      string // archlint | golangci | gofmt | gotest | jscpd
	name     string // clean | issues | mixed | violations | duplicates | needs-format | large-pass
	inputAbs string
}

func discoverScenarios(t *testing.T) []scenario {
	t.Helper()
	root, err := filepath.Abs(fixturesRoot)
	if err != nil {
		t.Fatalf("abs fixtures: %v", err)
	}
	var out []scenario
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !strings.Contains(base, ".input.") {
			return nil
		}
		dir := filepath.Base(filepath.Dir(path))
		name, _, _ := strings.Cut(base, ".input.")
		out = append(out, scenario{
			dir:      dir,
			name:     name,
			inputAbs: path,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk fixtures: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no fixtures discovered")
	}
	return out
}

// pipelineInput converts the fixture's raw bytes into something the main
// run() dispatch can consume. archlint/jscpd/gofmt require wrapping first
// (their inputs are tool-native, not SARIF).
func pipelineInput(t *testing.T, sc scenario) []byte {
	t.Helper()
	raw, err := os.ReadFile(sc.inputAbs)
	if err != nil {
		t.Fatalf("read fixture %s: %v", sc.inputAbs, err)
	}
	switch sc.dir {
	case "golangci", "gotest":
		return raw
	case subArchlint:
		return wrapToSARIF(t, []string{subWrap, subArchlint}, raw)
	case subJSCPD:
		return wrapToSARIF(t, []string{subWrap, subJSCPD}, raw)
	case subGofmt:
		return wrapToSARIF(t, []string{subWrap, subDiag, flagTool, subGofmt, flagRule, needsFormatRule}, raw)
	}
	t.Fatalf("unknown fixture dir %q", sc.dir)
	return nil
}

func wrapToSARIF(t *testing.T, args []string, in []byte) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, bytes.NewReader(in), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("wrap %v exit=%d stderr=%s", args, code, stderr.String())
	}
	return stdout.Bytes()
}

func TestE2E_Pipeline_ContractSurface(t *testing.T) {
	scenarios := discoverScenarios(t)
	formats := []string{"human", formatLLM, formatJSON}

	for _, sc := range scenarios {
		input := pipelineInput(t, sc)
		for _, fmtName := range formats {
			t.Run(sc.dir+"/"+sc.name+"/"+fmtName, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := run([]string{flagFormat, fmtName, flagNoState}, bytes.NewReader(input), &stdout, &stderr)

				if code != 0 && code != 1 {
					t.Fatalf("unexpected exit=%d; stderr=%s", code, stderr.String())
				}

				out := stdout.Bytes()
				if len(out) == 0 {
					t.Fatalf("empty output (exit=%d)", code)
				}

				switch fmtName {
				case formatLLM:
					if bytes.Contains(out, []byte("\x1b[")) {
						t.Errorf("llm output contains ANSI escapes")
					}
				case formatJSON:
					if bytes.Contains(out, []byte("\x1b[")) {
						t.Errorf("json output contains ANSI escapes")
					}
					var v any
					if err := json.Unmarshal(out, &v); err != nil {
						t.Errorf("json output not valid JSON: %v", err)
					}
				}
			})
		}
	}
}

func TestE2E_Pipeline_Determinism(t *testing.T) {
	scenarios := discoverScenarios(t)
	formats := []string{formatLLM}

	for _, sc := range scenarios {
		input := pipelineInput(t, sc)
		for _, fmtName := range formats {
			t.Run(sc.dir+"/"+sc.name+"/"+fmtName, func(t *testing.T) {
				a := runOnce(t, fmtName, input)
				b := runOnce(t, fmtName, input)
				if !bytes.Equal(a, b) {
					t.Fatalf("nondeterministic output for %s/%s/%s", sc.dir, sc.name, fmtName)
				}
			})
		}
	}
}

func runOnce(t *testing.T, fmtName string, input []byte) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	_ = run([]string{flagFormat, fmtName, flagNoState}, bytes.NewReader(input), &stdout, &stderr)
	return stdout.Bytes()
}

func TestE2E_WrapSubcommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
		dir  string
		ext  string
	}{
		{subArchlint, []string{subWrap, subArchlint}, subArchlint, ".input.json"},
		{subJSCPD, []string{subWrap, subJSCPD}, subJSCPD, ".input.json"},
		{"diag-gofmt", []string{subWrap, subDiag, flagTool, subGofmt, flagRule, needsFormatRule}, subGofmt, ".input.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureDir, err := filepath.Abs(filepath.Join(fixturesRoot, tc.dir))
			if err != nil {
				t.Fatal(err)
			}
			matches, err := filepath.Glob(filepath.Join(fixtureDir, "*"+tc.ext))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) == 0 {
				t.Fatalf("no fixtures in %s matching *%s", fixtureDir, tc.ext)
			}
			for _, in := range matches {
				t.Run(filepath.Base(in), func(t *testing.T) {
					raw, err := os.ReadFile(in)
					if err != nil {
						t.Fatalf("read %s: %v", in, err)
					}
					var stdout, stderr bytes.Buffer
					code := run(tc.args, bytes.NewReader(raw), &stdout, &stderr)
					if code != 0 {
						t.Fatalf("wrap exit=%d stderr=%s", code, stderr.String())
					}
					var probe struct {
						Version string            `json:"version"`
						Runs    []json.RawMessage `json:"runs"`
					}
					if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
						t.Fatalf("wrap output not JSON: %v", err)
					}
					if probe.Version != "2.1.0" {
						t.Fatalf("wrap output version=%q (want 2.1.0)", probe.Version)
					}
					if probe.Runs == nil {
						t.Fatalf("wrap output missing runs")
					}
				})
			}
		})
	}
}

func TestE2E_LLMGoldens(t *testing.T) {
	scenarios := discoverScenarios(t)
	for _, sc := range scenarios {
		t.Run(sc.dir+"/"+sc.name, func(t *testing.T) {
			input := pipelineInput(t, sc)
			var stdout, stderr bytes.Buffer
			_ = run([]string{flagFormat, formatLLM, flagNoState}, bytes.NewReader(input), &stdout, &stderr)
			goldenPath := filepath.Join(filepath.Dir(sc.inputAbs), sc.name+".llm.golden")
			if os.Getenv("UPDATE_LLM_GOLDENS") == "1" {
				if err := os.WriteFile(goldenPath, stdout.Bytes(), 0o644); err != nil {
					t.Fatalf("write golden %s: %v", goldenPath, err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}
			if !bytes.Equal(stdout.Bytes(), want) {
				t.Errorf("llm output drift for %s/%s\n--- want (%d bytes)\n%s\n--- got (%d bytes)\n%s",
					sc.dir, sc.name, len(want), string(want), stdout.Len(), stdout.String())
			}
		})
	}
}

// TestE2E_LLMDiffGolden verifies that the full LLM output — leaderboard then
// NEW block — matches a checked-in golden when a prior state is present.
// Run with UPDATE_LLM_DIFF_GOLDEN=1 to regenerate.
func TestE2E_LLMDiffGolden(t *testing.T) {
	priorState := `{"version":1,"runs":[{"generated_at":"2026-01-01T00:00:00Z","findings":{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":"error"}}]}`

	raw, err := os.ReadFile(fixturesRoot + "/golangci/issues.input.sarif")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	dir := t.TempDir()
	stateFile := filepath.Join(dir, "fo-state.json")
	if err := os.WriteFile(stateFile, []byte(priorState), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{flagFormat, formatLLM, flagStateFile, stateFile}, bytes.NewReader(raw), &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("unexpected exit=%d stderr=%s", code, stderr.String())
	}

	out := stdout.Bytes()

	if bytes.Contains(out, []byte("\x1b[")) {
		t.Errorf("llm output contains ANSI escapes")
	}
	if !bytes.Contains(out, []byte("\nNEW (")) {
		t.Errorf("expected NEW block in diff output; got:\n%s", out)
	}

	goldenPath := fixturesRoot + "/golangci/issues-withdiff.llm.golden"
	if os.Getenv("UPDATE_LLM_DIFF_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden written: %s (%d bytes)", goldenPath, len(out))
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s (run with UPDATE_LLM_DIFF_GOLDEN=1 to create): %v", goldenPath, err)
	}
	if !bytes.Equal(out, want) {
		t.Errorf("llm+diff output drift\n--- want (%d bytes)\n%s\n--- got (%d bytes)\n%s",
			len(want), string(want), len(out), string(out))
	}
}

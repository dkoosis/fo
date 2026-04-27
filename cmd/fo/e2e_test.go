// E2E tests against the v2 golden fixture suite — fixture-driven smoke,
// determinism, wrap subcommand checks, and llm byte-equivalence regression
// guard.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixturesRoot = "../../testdata/golden/v2"

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
		// Match *.input.* but not *.golden
		if !strings.Contains(base, ".input.") {
			return nil
		}
		dir := filepath.Base(filepath.Dir(path))
		// scenario name = part before ".input."
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
// (their inputs are tool-native, not SARIF). Returns the bytes that should
// be piped into run() for the contract test.
func pipelineInput(t *testing.T, sc scenario) []byte {
	t.Helper()
	raw, err := os.ReadFile(sc.inputAbs)
	if err != nil {
		t.Fatalf("read fixture %s: %v", sc.inputAbs, err)
	}
	switch sc.dir {
	case "golangci", "gotest":
		return raw
	case "archlint":
		return wrapToSARIF(t, []string{"wrap", "archlint"}, raw)
	case "jscpd":
		return wrapToSARIF(t, []string{"wrap", "jscpd"}, raw)
	case "gofmt":
		return wrapToSARIF(t, []string{"wrap", "diag", "--tool", "gofmt", "--rule", "needs-formatting"}, raw)
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

// TestE2E_Pipeline_ContractSurface walks every fixture, pipes it through
// the full v2 dispatch in each output format, and asserts the contract.
func TestE2E_Pipeline_ContractSurface(t *testing.T) {
	scenarios := discoverScenarios(t)
	formats := []string{"human", "llm", "json"}

	for _, sc := range scenarios {
		input := pipelineInput(t, sc)
		for _, fmtName := range formats {
			t.Run(sc.dir+"/"+sc.name+"/"+fmtName, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := run([]string{"--format", fmtName, "--no-state"}, bytes.NewReader(input), &stdout, &stderr)

				// Exit code contract: 0 (clean / no errors) or 1 (errors
				// or test failures). Anything else means dispatch failure.
				if code != 0 && code != 1 {
					t.Fatalf("unexpected exit=%d; stderr=%s", code, stderr.String())
				}

				out := stdout.Bytes()
				if len(out) == 0 {
					t.Fatalf("empty output (exit=%d)", code)
				}

				switch fmtName {
				case "llm":
					if bytes.Contains(out, []byte("\x1b[")) {
						t.Errorf("llm output contains ANSI escapes")
					}
				case "json":
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

// TestE2E_Pipeline_Determinism runs each fixture twice in llm format and
// asserts byte-equal output across runs. JSON is excluded — its envelope
// embeds a GeneratedAt timestamp by design.
func TestE2E_Pipeline_Determinism(t *testing.T) {
	scenarios := discoverScenarios(t)
	formats := []string{"llm"}

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
	_ = run([]string{"--format", fmtName, "--no-state"}, bytes.NewReader(input), &stdout, &stderr)
	return stdout.Bytes()
}

// TestE2E_WrapSubcommands exercises `fo wrap <name>` for each wrapper
// against its corresponding v1 fixtures. Every produced document must be
// SARIF 2.1.0.
func TestE2E_WrapSubcommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
		dir  string
		ext  string
	}{
		{"archlint", []string{"wrap", "archlint"}, "archlint", ".input.json"},
		{"jscpd", []string{"wrap", "jscpd"}, "jscpd", ".input.json"},
		{"diag-gofmt", []string{"wrap", "diag", "--tool", "gofmt", "--rule", "needs-formatting"}, "gofmt", ".input.txt"},
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

// TestE2E_LLMGoldens asserts byte-equivalence between live llm output and
// checked-in v2 goldens. llm is deterministic (no timestamp); regressions
// here mean the format drifted unintentionally.
func TestE2E_LLMGoldens(t *testing.T) {
	scenarios := discoverScenarios(t)
	for _, sc := range scenarios {
		t.Run(sc.dir+"/"+sc.name, func(t *testing.T) {
			input := pipelineInput(t, sc)
			var stdout, stderr bytes.Buffer
			_ = run([]string{"--format", "llm", "--no-state"}, bytes.NewReader(input), &stdout, &stderr)
			goldenPath := filepath.Join(filepath.Dir(sc.inputAbs), sc.name+".llm.golden")
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

// Sanity: a no-input invocation should fail with usage error (exit 2).
func TestE2E_NoInputIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, bytes.NewReader(nil), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("want exit 2, got %d (stderr=%s)", code, stderr.String())
	}
}

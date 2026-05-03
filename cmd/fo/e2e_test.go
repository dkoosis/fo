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

// Sanity: a no-input invocation should fail with usage error (exit 2).
func TestE2E_TallyPipeline(t *testing.T) {
	// wrap leaderboard → fo, end-to-end. Mirrors the trixi `make audit`
	// usage: `sort | uniq -c` style input flows through a single pipe
	// and renders as a Leaderboard. Exit code is always 0 (informational).
	tallyIn := "  14332 log.friction\n   2578 journal.day\n    701 log.session\n"

	wrapOut := runWrapForTest(t, tallyIn)
	if !bytes.HasPrefix(wrapOut.Bytes(), []byte("# fo:tally tool=dk-types\n")) {
		t.Errorf("missing tally header: %q", wrapOut.String())
	}

	var renderOut, renderErr bytes.Buffer
	if code := run([]string{"--format", "llm", "--no-state"}, &wrapOut, &renderOut, &renderErr); code != 0 {
		t.Fatalf("fo render: code=%d stderr=%s", code, renderErr.String())
	}
	out := renderOut.String()
	for _, want := range []string{"log.friction", "14332", "journal.day", "2578"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q:\n%s", want, out)
		}
	}

	jsonSrc := runWrapForTest(t, tallyIn)
	var jsonOut, jsonErr bytes.Buffer
	if code := run([]string{"--format", "json", "--no-state"}, &jsonSrc, &jsonOut, &jsonErr); code != 0 {
		t.Fatalf("fo json: code=%d stderr=%s", code, jsonErr.String())
	}
	if !bytes.Contains(jsonOut.Bytes(), []byte(`"tool": "dk-types"`)) {
		t.Errorf("json missing tool field: %s", jsonOut.String())
	}
	if !bytes.Contains(jsonOut.Bytes(), []byte(`"value": 14332`)) {
		t.Errorf("json missing value: %s", jsonOut.String())
	}
}

func runWrapForTest(t *testing.T, tallyIn string) bytes.Buffer {
	t.Helper()
	var out, errBuf bytes.Buffer
	if code := run([]string{"wrap", "leaderboard", "--tool", "dk-types"}, bytes.NewReader([]byte(tallyIn)), &out, &errBuf); code != 0 {
		t.Fatalf("wrap leaderboard: code=%d stderr=%s", code, errBuf.String())
	}
	return out
}

func TestE2E_NoInputIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, bytes.NewReader(nil), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("want exit 2, got %d (stderr=%s)", code, stderr.String())
	}
}

func TestE2E_Multiplex_SARIFAndTestjson(t *testing.T) {
	sarifBody := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"vet"}},"results":[{"ruleId":"R1","level":"error","message":{"text":"boom"}}]}]}`
	tjBody := `{"Time":"2026-04-27T15:00:00Z","Action":"fail","Package":"foo","Test":"TestX"}`
	input := "--- tool:vet format:sarif ---\n" + sarifBody + "\n" +
		"--- tool:test format:testjson ---\n" + tjBody + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader([]byte(input)), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("want exit 1 (failures present), got %d (stderr=%s)", code, stderr.String())
	}
	out := stdout.Bytes()
	if !bytes.Contains(out, []byte(`"tool": "multi"`)) {
		t.Errorf("expected merged report tool=multi, got: %s", out)
	}
	if !bytes.Contains(out, []byte(`"rule_id": "R1"`)) {
		t.Errorf("expected sarif finding in merged report, got: %s", out)
	}
	if !bytes.Contains(out, []byte(`"package": "foo"`)) {
		t.Errorf("expected testjson result in merged report, got: %s", out)
	}
}

func TestE2E_Multiplex_SectionParseFailure(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n" +
		"this is not json at all\n" +
		"--- tool:test format:testjson ---\n" +
		`{"Action":"pass","Package":"bar"}` + "\n"
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader([]byte(input)), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("want exit 1 (synthetic error finding), got %d (stderr=%s)", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"rule_id": "fo/section-parse-error"`)) {
		t.Errorf("expected synthetic parse-error finding, got: %s", stdout.String())
	}
}

// TestE2E_FixCommand_JSONOutput verifies that fix_command flows end-to-end
// from a wrapper (wrapdiag gofmt) through the pipeline into JSON output.
// This is the primary path Claude uses to get copy-pastable fix commands.
// TestE2E_SectionStatus_Timeout verifies that status:timeout on a delimiter
// with empty content produces a synthetic error finding rather than silently
// passing. This closes the "tool failed silently" ambiguity.
func TestE2E_SectionStatus_Timeout(t *testing.T) {
	input := "--- tool:govulncheck format:sarif status:timeout ---\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader([]byte(input)), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("want exit 1 (timeout is an error), got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("fo/section-timeout")) {
		t.Errorf("JSON output should contain fo/section-timeout finding:\n%s", stdout.String())
	}
}

func TestE2E_SectionStatus_Error(t *testing.T) {
	input := "--- tool:govulncheck format:sarif status:error ---\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader([]byte(input)), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("want exit 1 (error status), got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("fo/section-error")) {
		t.Errorf("JSON output should contain fo/section-error finding:\n%s", stdout.String())
	}
}

func TestE2E_SectionStatus_Skipped(t *testing.T) {
	// skipped tool with no content — should produce a note-level finding, exit 0
	input := "--- tool:govulncheck format:sarif status:skipped ---\n" +
		"--- tool:vet format:sarif status:ok ---\n" +
		`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"vet"}},"results":[]}]}` + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader([]byte(input)), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("want exit 0 (skipped is not a failure), got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("fo/section-skipped")) {
		t.Errorf("JSON output should contain fo/section-skipped finding:\n%s", stdout.String())
	}
}

func TestE2E_SectionStatus_OK_NoSyntheticFinding(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"vet"}},"results":[]}]}`
	input := "--- tool:vet format:sarif status:ok ---\n" + sarif + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader([]byte(input)), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("want exit 0 (ok status, clean), got %d stderr=%s", code, stderr.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte("fo/section-")) {
		t.Errorf("OK status should produce no synthetic findings:\n%s", stdout.String())
	}
}

func TestE2E_FixCommand_JSONOutput(t *testing.T) {
	raw, err := os.ReadFile(fixturesRoot + "/gofmt/needs-format.input.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sarif := wrapToSARIF(t, []string{"wrap", "diag", "--tool", "gofmt", "--rule", "needs-formatting"}, raw)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", "--no-state"}, bytes.NewReader(sarif), &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("unexpected exit %d stderr=%s", code, stderr.String())
	}

	out := stdout.Bytes()
	if !bytes.Contains(out, []byte(`"fix_command"`)) {
		t.Errorf("JSON output missing fix_command field:\n%s", out)
	}
	if !bytes.Contains(out, []byte(`gofmt -w`)) {
		t.Errorf("JSON output fix_command should contain gofmt -w:\n%s", out)
	}
}

// TestE2E_LLMDiffGolden verifies that the full LLM output — leaderboard then
// NEW block — matches a checked-in golden when a prior state is present.
// Prior state contains one dummy fingerprint not in the current run (so it
// shows as RESOLVED) and no current fingerprints (so all current findings are
// NEW). Run with UPDATE_LLM_DIFF_GOLDEN=1 to regenerate the golden.
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
	code := run([]string{"--format", "llm", "--state-file", stateFile}, bytes.NewReader(raw), &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("unexpected exit=%d stderr=%s", code, stderr.String())
	}

	out := stdout.Bytes()

	// Structure assertions — independent of golden content.
	if bytes.Contains(out, []byte("\x1b[")) {
		t.Errorf("llm output contains ANSI escapes")
	}
	if !bytes.Contains(out, []byte("\nNEW (")) {
		t.Errorf("expected NEW block in diff output; got:\n%s", out)
	}

	// Golden comparison.
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

// TestE2E_FixCommand_LLMOutput verifies fix: hint lines appear in LLM output
// for wrappers that emit fix commands (gofmt path).
func TestE2E_FixCommand_LLMOutput(t *testing.T) {
	raw, err := os.ReadFile(fixturesRoot + "/gofmt/needs-format.input.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sarif := wrapToSARIF(t, []string{"wrap", "diag", "--tool", "gofmt", "--rule", "needs-formatting"}, raw)

	var stdout, stderr bytes.Buffer
	_ = run([]string{"--format", "llm", "--no-state"}, bytes.NewReader(sarif), &stdout, &stderr)

	out := stdout.Bytes()
	if !bytes.Contains(out, []byte("fix: gofmt -w")) {
		t.Errorf("LLM output missing fix: hint:\n%s", out)
	}
	if bytes.Contains(out, []byte("\x1b[")) {
		t.Errorf("LLM output must not contain ANSI escapes")
	}
}

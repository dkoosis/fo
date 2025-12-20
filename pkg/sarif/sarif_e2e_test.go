package sarif

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helperEnvKey = "FO_E2E_HELPER"

func TestHelperProcess(t *testing.T) { //nolint:revive
	t.Helper()

	if os.Getenv(helperEnvKey) == "" {
		return
	}

	args := filterHelperArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "no helper scenario provided")
		os.Exit(2)
	}

	switch args[0] {
	case "sarif-output":
		path := extractValue(args[1:], "--sarif-path=", os.Getenv("FO_E2E_SARIF_PATH"))
		if path == "" {
			fmt.Fprintln(os.Stderr, "missing sarif path")
			os.Exit(3)
		}
		if err := os.WriteFile(path, []byte(sampleSARIFDocument()), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
		fmt.Println("sarif document written")
		os.Exit(0)
	case "sleep":
		sleepMs := parseInt(args[1:], "--sleep-ms=", 100)
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
		fmt.Println("slept")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper scenario: %s\n", args[0])
	}

	os.Exit(0)
}

func TestSarifOrchestrator_WritesDocumentsAndRendersSummaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	t.Setenv(helperEnvKey, "1")

	tempDir := t.TempDir()
	sarifPath := filepath.Join(tempDir, "e2e.sarif")

	config := DefaultRendererConfig()
	foConfig := design.UnicodeVibrantTheme()
	orch := NewOrchestrator(config, foConfig)
	orch.SetBuildDir(tempDir)

	var buf bytes.Buffer
	orch.SetWriter(&buf)

	command, args := helperCommand(t, "sarif-output", "--sarif-path="+sarifPath)

	results, err := orch.Run(ctx, []ToolSpec{
		{
			Name:      "demo-tool",
			Command:   command,
			Args:      args,
			SARIFPath: sarifPath,
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Document)

	assert.FileExists(t, sarifPath)

	data, readErr := os.ReadFile(sarifPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"ruleId": "E2E001"`)

	stats := ComputeStats(results[0].Document)
	assert.GreaterOrEqual(t, stats.TotalIssues, 1)

	output := buf.String()
	assert.Contains(t, output, "E2E001")
}

// --- Helper utilities duplicated locally to keep tests hermetic ---

func helperCommand(t *testing.T, scenario string, extraArgs ...string) (string, []string) {
	t.Helper()
	args := []string{"-test.run=TestHelperProcess", "--", scenario}
	args = append(args, extraArgs...)
	return os.Args[0], args
}

func filterHelperArgs(args []string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-test.") {
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

func extractValue(args []string, prefix, fallback string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return fallback
}

func parseInt(args []string, prefix string, fallback int) int {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			value := strings.TrimPrefix(arg, prefix)
			if n, err := strconv.Atoi(value); err == nil {
				return n
			}
		}
	}
	return fallback
}

func sampleSARIFDocument() string {
	return `{
  "version": "2.1.0",
  "$schema": "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.6.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "e2e-lint",
          "rules": [
            {
              "id": "E2E001",
              "shortDescription": { "text": "demo issue" }
            }
          ]
        }
      },
      "results": [
        {
          "ruleId": "E2E001",
          "level": "error",
          "message": { "text": "found a demo issue" },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": { "uri": "sample.go" },
                "region": { "startLine": 3, "startColumn": 1 }
              }
            }
          ]
        }
      ]
    }
  ]
}`
}

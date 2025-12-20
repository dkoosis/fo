package fo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const helperEnvKey = "FO_E2E_HELPER"

// helperCommand builds the command/args tuple for invoking the test helper process.
func helperCommand(t *testing.T, scenario string, extraArgs ...string) (string, []string) {
	t.Helper()

	args := []string{"-test.run=TestHelperProcess", "--", scenario}
	args = append(args, extraArgs...)
	return os.Args[0], args
}

// TestHelperProcess acts as a stand-in executable for end-to-end tests.
// It is only activated when the FO_E2E_HELPER environment variable is set.
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

	scenario := args[0]
	rest := args[1:]

	switch scenario {
	case "success":
		message := extractValue(rest, "--msg=", "helper success")
		fmt.Println(message)
		os.Exit(0)
	case "slow-success":
		sleepMs := parseInt(rest, "--sleep-ms=", 100)
		message := extractValue(rest, "--msg=", "slow helper completed")
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
		fmt.Println(message)
		os.Exit(0)
	case "failure":
		fmt.Fprintln(os.Stderr, "intentional failure from helper")
		os.Exit(3)
	case "go-test-json":
		exitCode := parseInt(rest, "--exit-code=", 0)
		for _, line := range sampleGoTestJSONLines() {
			fmt.Println(line)
		}
		os.Exit(exitCode)
	case "sarif-output":
		path := extractValue(rest, "--sarif-path=", os.Getenv("FO_E2E_SARIF_PATH"))
		if path == "" {
			fmt.Fprintln(os.Stderr, "missing sarif path")
			os.Exit(4)
		}
		if err := os.WriteFile(path, []byte(sampleSARIFDocument()), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(5)
		}
		fmt.Println("sarif document written")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper scenario: %s\n", scenario)
	}

	os.Exit(0)
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

func sampleGoTestJSONLines() []string {
	return []string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example/pkg","Test":"TestAlpha"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example/pkg","Test":"TestAlpha","Elapsed":0.01}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example/pkg","Test":"TestBeta"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example/pkg","Test":"TestBeta","Elapsed":0.02}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example/pkg","Output":"coverage: 82.0% of statements"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example/pkg","Elapsed":0.05}`,
	}
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

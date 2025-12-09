package sarif

// CommonTools returns ToolSpecs for commonly used Go analysis tools.
func CommonTools(buildDir string) []ToolSpec {
	return []ToolSpec{
		GolangciLintTool(buildDir),
	}
}

// GolangciLintTool returns a ToolSpec for golangci-lint with SARIF output.
func GolangciLintTool(buildDir string) ToolSpec {
	return ToolSpec{
		Name:    "golangci-lint",
		Command: "golangci-lint",
		Args: []string{
			"run",
			"--output.sarif.path=" + buildDir + "/golangci-lint.sarif",
			"./...",
		},
		SARIFPath: buildDir + "/golangci-lint.sarif",
	}
}

// GoVetTool returns a ToolSpec for go vet.
// Note: go vet doesn't have native SARIF output, so this uses JSON
// and would need a converter. Included for future extension.
func GoVetTool(buildDir string) ToolSpec {
	return ToolSpec{
		Name:    "go-vet",
		Command: "go",
		Args: []string{
			"vet",
			"-json",
			"./...",
		},
		SARIFPath: buildDir + "/go-vet.sarif", // Would need conversion
	}
}

// GoTestTool returns a ToolSpec for go test with JSON output.
// Note: Tests use a different output format, included for completeness.
func GoTestTool(buildDir string) ToolSpec {
	return ToolSpec{
		Name:    "go-test",
		Command: "go",
		Args: []string{
			"test",
			"-json",
			"./...",
		},
		SARIFPath: buildDir + "/go-test.sarif", // Would need conversion
	}
}

// CustomTool creates a ToolSpec for any command that produces SARIF.
func CustomTool(name, command string, args []string, sarifPath string) ToolSpec {
	return ToolSpec{
		Name:      name,
		Command:   command,
		Args:      args,
		SARIFPath: sarifPath,
	}
}

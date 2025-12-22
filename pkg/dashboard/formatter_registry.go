package dashboard

import "strings"

// FormatterRegistry holds registered formatters.
var formatters = []OutputFormatter{
	&RaceFormatter{}, // Must be before GoTestFormatter (more specific match)
	&GoTestFormatter{},
	&FilesizeDashboardFormatter{}, // Must be before SARIF to match dashboard format
	&MCPErrorsFormatter{},         // mcp-errors -format=dashboard output
	&NugstatsFormatter{},          // nugstats -format=dashboard output
	&OrcaHygieneFormatter{},       // orca-hygiene -format=dashboard output
	&TelemetrySignalsFormatter{},  // telemetry-signals -format=dashboard output
	&GovulncheckFormatter{},       // govulncheck output
	&GolangciLintFormatter{},      // Per-linter sections for golangci-lint
	&GofmtFormatter{},             // gofmt -l output
	&GoVetFormatter{},             // go vet output
	&GoBuildFormatter{},           // go build output
	&GoArchLintFormatter{},        // go-arch-lint output
	&NilawayFormatter{},           // nilaway -json output
	&SARIFFormatter{},
	&JscpdFormatter{},             // jscpd --reporters console output
	&PlainFormatter{}, // fallback, always last
}

// FormatOutput selects the appropriate formatter and formats the output.
func FormatOutput(command string, lines []string, width int) string {
	for _, f := range formatters {
		if f.Matches(command) {
			return f.Format(lines, width)
		}
	}
	return strings.Join(lines, "\n")
}

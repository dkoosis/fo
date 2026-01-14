package dashboard

import "strings"

// FormatterRegistry holds registered formatters.
var formatters = []OutputFormatter{
	&RaceFormatter{}, // Must be before GoTestFormatter (more specific match)
	&GoTestFormatter{},
	&FilesizeDashboardFormatter{}, // Must be before SARIF to match dashboard format
	&SnipeMetricsFormatter{},      // snipe BASELINE.json output
	&KGBaselineFormatter{},        // kg-baseline.json output
	&MCPErrorsFormatter{},         // mcp-logscan -format=dashboard output
	&NugstatsFormatter{},          // nugstats -format=dashboard output
	&OrcaHygieneFormatter{},       // orca-hygiene -format=dashboard output
	&HTMLHygieneFormatter{},       // html-export-hygiene -format=dashboard output
	&TelemetrySignalsFormatter{},  // telemetry-signals -format=dashboard output
	&GovulncheckFormatter{},       // govulncheck output
	&GolangciLintFormatter{},      // Per-linter sections for golangci-lint
	&GofmtFormatter{},             // gofmt -l output
	&GoVetFormatter{},             // go vet output
	&GoBuildFormatter{},           // go build output
	&GoArchLintFormatter{},        // go-arch-lint output
	&NilawayFormatter{},           // nilaway -json output
	&SARIFFormatter{},
	&JscpdFormatter{}, // jscpd --reporters console output
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

// GetFormatter returns the formatter for the given command, or nil if none matches.
func GetFormatter(command string) OutputFormatter {
	for _, f := range formatters {
		if f.Matches(command) {
			return f
		}
	}
	return nil
}

// GetIndicatorStatus returns the status indicator for a task based on its output.
// If the formatter implements StatusIndicator, it delegates to GetStatus.
// Otherwise returns IndicatorDefault.
func GetIndicatorStatus(command string, lines []string) IndicatorStatus {
	f := GetFormatter(command)
	if f == nil {
		return IndicatorDefault
	}
	if si, ok := f.(StatusIndicator); ok {
		return si.GetStatus(lines)
	}
	return IndicatorDefault
}

// FormatterPrefersBatch returns true if the formatter for this command
// prefers batch mode (output collected before formatting, no streaming).
func FormatterPrefersBatch(command string) bool {
	f := GetFormatter(command)
	if f == nil {
		return false
	}
	if bf, ok := f.(BatchFormatter); ok {
		return bf.PrefersBatch()
	}
	return false
}

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dkoosis/fo/fo"
	config "github.com/dkoosis/fo/internal/config"
	"github.com/dkoosis/fo/internal/version"
	"github.com/dkoosis/fo/pkg/archlint"
	"github.com/dkoosis/fo/pkg/dashboard"
	"github.com/dkoosis/fo/pkg/design"
	"github.com/dkoosis/fo/pkg/fuzz"
	"github.com/dkoosis/fo/pkg/gofmt"
	"github.com/dkoosis/fo/pkg/goleak"
	"github.com/dkoosis/fo/pkg/nilaway"
	"github.com/dkoosis/fo/pkg/racedetect"
	"github.com/dkoosis/fo/pkg/sarif"
	"golang.org/x/term"
)

// validPatterns defines the supported pattern names for the --pattern flag.
var validPatterns = map[string]bool{
	"test-table":  true,
	"sparkline":   true,
	"leaderboard": true,
	"inventory":   true,
	"summary":     true,
	"comparison":  true,
}

// formatHandler encapsulates format detection and rendering logic.
type formatHandler struct {
	detect func([]byte) bool
	render func([]byte, *design.Config) int
}

// formatHandlers defines the ordered list of format handlers.
// Order matters: first matching handler wins.
var formatHandlers = []formatHandler{
	{detect: sarif.IsSARIF, render: renderSARIF},
	{detect: archlint.IsArchLintJSON, render: renderArchLint},
	{detect: gofmt.IsGofmtOutput, render: renderGofmt},
	{detect: nilaway.IsNilawayOutput, render: renderNilaway},
	{detect: goleak.IsGoleakOutput, render: renderGoleak},
	{detect: racedetect.IsRaceDetectorOutput, render: renderRaceDetector},
	{detect: fuzz.IsFuzzOutput, render: renderFuzz},
}

// LocalAppConfig holds behavioral settings derived from AppConfig and CLI flags.
type LocalAppConfig struct {
	Label            string
	LiveStreamOutput bool
	ShowOutput       string
	NoTimer          bool // Effective NoTimer after all flags/configs
	NoColor          bool // Effective NoColor (IsMonochrome)
	CI               bool // Effective CI mode
	Debug            bool
	MaxBufferSize    int64 // Max total size for combined stdout/stderr in capture mode
	MaxLineLength    int   // Max size for a single line from stdout/stderr
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// subcommand represents a named subcommand with its handler.
type subcommand struct {
	name string
	run  func(args []string) int
}

// subcommands is the dispatch table for known subcommands.
var subcommands = map[string]subcommand{
	"print":  {name: "print", run: runPrintSubcommand},
	"replay": {name: "replay", run: handleReplayCommand},
}

// run executes the application logic and returns the exit code.
// This allows integration tests to invoke the logic without os.Exit() terminating the test runner.
func run(args []string) int {
	// Check for subcommand first
	if len(args) > 1 {
		cmdName := args[1]
		if !strings.HasPrefix(cmdName, "-") {
			if sub, ok := subcommands[cmdName]; ok {
				return sub.run(args[2:])
			}
		}
	}

	// Parse global flags using FlagSet (no os.Args mutation)
	cliFlags, versionFlag, cmdArgs, err := parseGlobalFlagsFromArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] %v\n", err)
		return 1
	}

	// Handle version flag
	if versionFlag {
		_, _ = fmt.Fprintf(os.Stdout, "fo version %s\n", version.Version)
		_, _ = fmt.Fprintf(os.Stdout, "Commit: %s\n", version.CommitHash)
		_, _ = fmt.Fprintf(os.Stdout, "Built: %s\n", version.BuildDate)
		return 0
	}

	if cliFlags.Dashboard {
		return runDashboardMode(cliFlags)
	}

	// Load application configuration from .fo.yaml
	fileAppConfig := config.LoadConfig()
	if len(cmdArgs) == 0 {
		// No command specified - check if data is being piped via stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped in - run Editor mode
			return runEditorMode(cliFlags, fileAppConfig)
		}
		// No command and no pipe - show usage
		fmt.Fprintln(os.Stderr, "[fo] Error: No command specified")
		fmt.Fprintln(os.Stderr, "[fo] Usage: fo [flags] -- <COMMAND> [ARGS...]")
		fmt.Fprintln(os.Stderr, "[fo]        <command> | fo [flags]")
		return 1
	}

	// Apply any command-specific presets from the config file
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(fileAppConfig, cmdArgs[0])
	}

	// Convert the file-based AppConfig to LocalAppConfig for runtime behavior
	behavioralSettings := convertAppConfigToLocal(fileAppConfig)

	// Override behavioral settings with any explicitly set CLI flags
	if cliFlags.Label != "" {
		behavioralSettings.Label = cliFlags.Label
	}
	if cliFlags.LiveStreamOutputSet {
		behavioralSettings.LiveStreamOutput = cliFlags.LiveStreamOutput
	}
	if cliFlags.ShowOutputSet && cliFlags.ShowOutput != "" {
		behavioralSettings.ShowOutput = cliFlags.ShowOutput
	}

	// Debug is ONLY enabled by explicit --debug flag
	if cliFlags.DebugSet {
		behavioralSettings.Debug = cliFlags.Debug
		fileAppConfig.Debug = cliFlags.Debug
	} else {
		// Force debug off unless explicitly enabled by flag
		behavioralSettings.Debug = false
		fileAppConfig.Debug = false
	}

	if cliFlags.MaxBufferSize > 0 {
		behavioralSettings.MaxBufferSize = cliFlags.MaxBufferSize
	}
	if cliFlags.MaxLineLength > 0 {
		behavioralSettings.MaxLineLength = cliFlags.MaxLineLength
	}

	// Resolve configuration from all sources with explicit priority order
	// Priority: CLI flags > Environment variables > .fo.yaml > Defaults
	resolvedCfg, err := config.ResolveConfig(cliFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error resolving configuration: %v\n", err)
		return 1
	}
	finalDesignConfig := resolvedCfg.Theme

	// Update behavioralSettings from resolved config
	behavioralSettings.NoTimer = resolvedCfg.NoTimer
	behavioralSettings.NoColor = resolvedCfg.NoColor
	behavioralSettings.CI = resolvedCfg.CI
	behavioralSettings.Debug = resolvedCfg.Debug
	behavioralSettings.LiveStreamOutput = resolvedCfg.LiveStreamOutput
	behavioralSettings.ShowOutput = resolvedCfg.ShowOutput
	behavioralSettings.MaxBufferSize = resolvedCfg.MaxBufferSize
	behavioralSettings.MaxLineLength = resolvedCfg.MaxLineLength

	consoleCfg := fo.ConsoleConfig{
		ThemeName:        finalDesignConfig.ThemeName,
		UseBoxes:         finalDesignConfig.Style.UseBoxes,
		UseBoxesSet:      true,
		Monochrome:       finalDesignConfig.IsMonochrome,
		ShowTimer:        !finalDesignConfig.Style.NoTimer,
		ShowTimerSet:     true,
		ShowOutputMode:   behavioralSettings.ShowOutput,
		LiveStreamOutput: behavioralSettings.LiveStreamOutput,
		PatternHint:      cliFlags.PatternHint,
		Debug:            behavioralSettings.Debug,
		MaxBufferSize:    behavioralSettings.MaxBufferSize,
		MaxLineLength:    behavioralSettings.MaxLineLength,
		Design:           finalDesignConfig,
	}

	console := fo.NewConsole(consoleCfg)
	result, err := console.Run(behavioralSettings.Label, cmdArgs[0], cmdArgs[1:]...)

	exitCode := 0
	if result != nil {
		exitCode = result.ExitCode
	}
	if err != nil && result == nil {
		exitCode = 1
	}

	// Output JSON format if requested
	if cliFlags.Format == "json" && result != nil {
		jsonOutput, jsonErr := result.ToJSON()
		if jsonErr != nil {
			fmt.Fprintf(os.Stderr, "[fo] Error generating JSON output: %v\n", jsonErr)
			return 1
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", string(jsonOutput))
		return exitCode
	}

	if behavioralSettings.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG run()] returning exit code %d.\nBehavioral Config: %+v\n", exitCode, behavioralSettings)
	}
	return exitCode
}

// main is the entry point of the application.
// It calls run() and exits with the returned exit code.
func main() {
	os.Exit(run(os.Args))
}

// runDashboardMode orchestrates the dashboard workflow.
func runDashboardMode(cliFlags config.CliFlags) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	specs := make([]dashboard.TaskSpec, 0, len(cliFlags.Tasks))
	for _, raw := range cliFlags.Tasks {
		spec, err := dashboard.ParseTaskFlag(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fo] %v\n", err)
			return 1
		}
		specs = append(specs, spec)
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		manifestSpecs, err := dashboard.ParseManifest(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fo] %v\n", err)
			return 1
		}
		specs = append(specs, manifestSpecs...)
	}

	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "[fo] Error: No tasks provided for dashboard mode.")
		return 1
	}

	// Initialize dashboard theme from config
	dashboard.InitThemeFromConfig()

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return dashboard.RunNonTTY(ctx, specs, os.Stdout)
	}

	code, err := dashboard.RunDashboard(ctx, specs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] dashboard error: %v\n", err)
	}
	return code
}

func convertAppConfigToLocal(appCfg *config.AppConfig) LocalAppConfig {
	return LocalAppConfig{
		Label:            appCfg.Label,
		LiveStreamOutput: appCfg.LiveStreamOutput,
		ShowOutput:       appCfg.ShowOutput,
		NoTimer:          appCfg.NoTimer,
		NoColor:          appCfg.NoColor,
		CI:               appCfg.CI,
		Debug:            false, // Default to false, only enable when explicitly set by flag
		MaxBufferSize:    appCfg.MaxBufferSize,
		MaxLineLength:    appCfg.MaxLineLength,
	}
}

// findCommandArgsFromSlice extracts command arguments after "--" from the given args slice.
// Returns the command and its arguments, or empty slice if none found.
func findCommandArgsFromSlice(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			if i < len(args)-1 {
				return args[i+1:]
			}
			return []string{}
		}
	}
	return []string{}
}

// runEditorMode processes piped stdin input through fo's formatting pipeline.
// This is the "Editor" mode - fo receives messy output and transforms it into
// clear, beautiful signal.
func runEditorMode(cliFlags config.CliFlags, _ *config.AppConfig) int {
	// Resolve configuration with priority order
	resolvedCfg, err := config.ResolveConfig(cliFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error resolving configuration: %v\n", err)
		return 1
	}

	// Build console config
	consoleCfg := fo.ConsoleConfig{
		ThemeName:        resolvedCfg.Theme.ThemeName,
		UseBoxes:         resolvedCfg.Theme.Style.UseBoxes,
		UseBoxesSet:      true,
		Monochrome:       resolvedCfg.Theme.IsMonochrome,
		ShowTimer:        false, // No timer for stdin processing
		ShowTimerSet:     true,
		ShowOutputMode:   "always",
		LiveStreamOutput: cliFlags.LiveStreamOutput,
		PatternHint:      cliFlags.PatternHint,
		Debug:            resolvedCfg.Debug,
		MaxBufferSize:    resolvedCfg.MaxBufferSize,
		MaxLineLength:    resolvedCfg.MaxLineLength,
		Design:           resolvedCfg.Theme,
	}

	console := fo.NewConsole(consoleCfg)

	// Use live streaming mode if -s/--stream flag is set
	if cliFlags.LiveStreamOutput {
		if err := console.RunLive(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "[fo] Error processing stream: %v\n", err)
			return 1
		}
		return 0
	}

	// Batch mode: read all stdin, then process and render
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error reading stdin: %v\n", err)
		return 1
	}

	if len(input) == 0 {
		// Nothing to process
		return 0
	}

	// Try each format handler in order; first match wins
	for _, h := range formatHandlers {
		if h.detect(input) {
			return h.render(input, resolvedCfg.Theme)
		}
	}

	// No known format detected; fall through to generic processing
	// Create a task for the stdin processing
	task := design.NewTask("stdin", "stream", "pipe", nil, resolvedCfg.Theme)

	// Process through the processor
	console.ProcessStdin(task, input)

	// Render output
	for _, line := range task.OutputLines {
		rendered := task.RenderOutputLine(line)
		fmt.Println(rendered)
	}

	return 0
}

// renderSARIF parses and renders SARIF input using the sarif renderer.
func renderSARIF(input []byte, theme *design.Config) int {
	doc, err := sarif.ReadBytes(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing SARIF: %v\n", err)
		return 1
	}

	// Use default SARIF renderer config
	rendererCfg := sarif.DefaultRendererConfig()
	renderer := sarif.NewRenderer(rendererCfg, theme)

	output := renderer.Render(doc)
	if output != "" {
		fmt.Print(output)
	}

	return 0
}

// renderArchLint parses and renders go-arch-lint JSON output.
func renderArchLint(input []byte, theme *design.Config) int {
	adapter := archlint.NewAdapter(theme)
	result, err := adapter.ParseBytes(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing go-arch-lint JSON: %v\n", err)
		return 1
	}

	output := adapter.Render(result)
	if output != "" {
		fmt.Print(output)
	}

	// Return non-zero if there were violations
	if result.Payload.ArchHasWarnings {
		return 1
	}
	return 0
}

// renderGofmt parses and renders gofmt -l output.
func renderGofmt(input []byte, theme *design.Config) int {
	adapter := gofmt.NewAdapter(theme)
	result, err := adapter.ParseString(string(input))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing gofmt output: %v\n", err)
		return 1
	}

	output := adapter.Render(result)
	if output != "" {
		fmt.Print(output)
	}

	// Return non-zero if files need formatting
	if len(result.Files) > 0 {
		return 1
	}
	return 0
}

// renderNilaway parses and renders nilaway JSON output.
func renderNilaway(input []byte, theme *design.Config) int {
	adapter := nilaway.NewAdapter(theme)
	result, err := adapter.ParseBytes(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing nilaway output: %v\n", err)
		return 1
	}

	output := adapter.Render(result)
	if output != "" {
		fmt.Print(output)
	}

	// Return non-zero if there were findings
	if len(result.Findings) > 0 {
		return 1
	}
	return 0
}

// renderGoleak parses and renders goleak output.
func renderGoleak(input []byte, theme *design.Config) int {
	adapter := goleak.NewAdapter(theme)
	result, err := adapter.ParseBytes(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing goleak output: %v\n", err)
		return 1
	}

	output := adapter.Render(result)
	if output != "" {
		fmt.Print(output)
	}

	// Return non-zero if there were leaked goroutines
	if len(result.Goroutines) > 0 {
		return 1
	}
	return 0
}

// renderRaceDetector parses and renders race detector output.
func renderRaceDetector(input []byte, theme *design.Config) int {
	adapter := racedetect.NewAdapter(theme)
	result, err := adapter.ParseBytes(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing race detector output: %v\n", err)
		return 1
	}

	output := adapter.Render(result)
	if output != "" {
		fmt.Print(output)
	}

	// Return non-zero if there were data races
	if len(result.Races) > 0 {
		return 1
	}
	return 0
}

// renderFuzz parses and renders fuzz testing output.
func renderFuzz(input []byte, theme *design.Config) int {
	adapter := fuzz.NewAdapter(theme)
	result, err := adapter.ParseBytes(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing fuzz output: %v\n", err)
		return 1
	}

	output := adapter.Render(result)
	if output != "" {
		fmt.Print(output)
	}

	// Return non-zero if there were crashes
	if len(result.Failures) > 0 {
		return 1
	}
	return 0
}

// runPrintSubcommand handles the "fo print" subcommand.
func runPrintSubcommand(args []string) int {
	printFlagSet := flag.NewFlagSet("print", flag.ExitOnError)
	typeFlag := printFlagSet.String("type", "info", "Type of message (info, success, warning, error, header, raw)")
	iconFlag := printFlagSet.String("icon", "", "Custom icon to use (overrides type default)")
	indentFlag := printFlagSet.Int("indent", 0, "Number of indentation levels")
	// Global flags that should also apply to 'print'
	themeFlag := printFlagSet.String("theme", "", "Select visual theme")
	noColorFlag := printFlagSet.Bool("no-color", false, "Disable ANSI color/styling output for print")
	ciFlag := printFlagSet.Bool("ci", false, "Enable CI-friendly, plain-text output for print")
	debugFlag := printFlagSet.Bool("debug", false, "Enable debug output for print processing")

	// Parse print-specific flags
	err := printFlagSet.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error parsing 'print' flags: %v\n", err)
		return 1
	}
	messageParts := printFlagSet.Args()
	message := strings.Join(messageParts, " ")

	if message == "" && *typeFlag != "raw" { // Allow empty raw for just printing newline or control chars
		fmt.Fprintln(os.Stderr, "[fo] Error: No message provided for 'fo print'.")
		printFlagSet.Usage()
		return 1
	}

	// Create a config.CliFlags with just the print-relevant flags
	var globalCliFlagsForPrint config.CliFlags
	if *themeFlag != "" {
		globalCliFlagsForPrint.ThemeName = *themeFlag
	}
	if *noColorFlag {
		globalCliFlagsForPrint.NoColor = true
		globalCliFlagsForPrint.NoColorSet = true
	}
	if *ciFlag {
		globalCliFlagsForPrint.CI = true
		globalCliFlagsForPrint.CISet = true
	}
	if *debugFlag {
		globalCliFlagsForPrint.Debug = true
		globalCliFlagsForPrint.DebugSet = true
	}

	// Get the debug mode flag for local use
	debug := globalCliFlagsForPrint.Debug

	// Resolve configuration with priority order
	resolvedCfg, err := config.ResolveConfig(globalCliFlagsForPrint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo] Error resolving configuration: %v\n", err)
		return 1
	}
	finalDesignConfig := resolvedCfg.Theme

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG runPrintSubcommand] Type: %s, Icon: %s, Indent: %d, Message: '%s'\n",
			*typeFlag, *iconFlag, *indentFlag, message)
		fmt.Fprintf(os.Stderr, "[DEBUG runPrintSubcommand] finalDesignConfig.ThemeName: %s, IsMonochrome: %t\n",
			finalDesignConfig.ThemeName, finalDesignConfig.IsMonochrome)
	}

	// Use the new render function for direct messages
	output := design.RenderDirectMessage(finalDesignConfig, *typeFlag, *iconFlag, message, *indentFlag)
	_, _ = os.Stdout.WriteString(output) // Print directly to stdout
	return 0
}

// handleReplayCommand replays captured output through the adapter detection system.
// This is useful for debugging adapters and refining output rendering without re-running commands.
func handleReplayCommand(args []string) int {
	replayFlagSet := flag.NewFlagSet("replay", flag.ExitOnError)
	themeFlag := replayFlagSet.String("theme", "", "Select visual theme for replay")
	debugFlag := replayFlagSet.Bool("debug", false, "Enable debug output")
	patternFlag := replayFlagSet.String("pattern", "", "Force specific visualization pattern")
	showOnlyFlag := replayFlagSet.String("show", "all", "Show: all, stdout, stderr")

	replayFlagSet.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: fo replay [flags] <capture-file.json>")
		fmt.Fprintln(os.Stderr, "\nReplays captured command output through fo's adapter detection system.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		replayFlagSet.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  fo replay /tmp/fo-captures/2025-01-01_120000_Build.json")
		fmt.Fprintln(os.Stderr, "  fo replay --theme ascii_minimal capture.json")
		fmt.Fprintln(os.Stderr, "  fo replay --pattern test-table test-output.json")
	}

	if err := replayFlagSet.Parse(args); err != nil {
		return 1
	}

	remainingArgs := replayFlagSet.Args()
	if len(remainingArgs) == 0 {
		fmt.Fprintln(os.Stderr, "[fo replay] Error: No capture file specified")
		replayFlagSet.Usage()
		return 1
	}

	captureFile := remainingArgs[0]

	// Read capture file
	data, err := os.ReadFile(captureFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo replay] Error reading capture file: %v\n", err)
		return 1
	}

	// Parse capture JSON
	var capture struct {
		Args      []string `json:"args"`
		Command   string   `json:"command"`
		ExitCode  int      `json:"exit_code"`
		Label     string   `json:"label"`
		Stderr    string   `json:"stderr"`
		Stdout    string   `json:"stdout"`
		Timestamp string   `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &capture); err != nil {
		fmt.Fprintf(os.Stderr, "[fo replay] Error parsing capture file: %v\n", err)
		return 1
	}

	// Build CLI flags for config resolution
	var cliFlags config.CliFlags
	if *themeFlag != "" {
		cliFlags.ThemeName = *themeFlag
	}
	if *debugFlag {
		cliFlags.Debug = true
		cliFlags.DebugSet = true
	}
	if *patternFlag != "" {
		if !validPatterns[*patternFlag] {
			fmt.Fprintf(os.Stderr, "[fo replay] Error: Invalid pattern '%s'\n", *patternFlag)
			return 1
		}
		cliFlags.PatternHint = *patternFlag
		cliFlags.PatternHintSet = true
	}

	// Resolve configuration
	resolvedCfg, err := config.ResolveConfig(cliFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fo replay] Error resolving configuration: %v\n", err)
		return 1
	}

	// Print replay header
	fmt.Printf("─── Replaying: %s ───\n", capture.Label)
	fmt.Printf("Command: %s %s\n", capture.Command, strings.Join(capture.Args, " "))
	fmt.Printf("Captured: %s\n", capture.Timestamp)
	fmt.Printf("Exit code: %d\n", capture.ExitCode)
	fmt.Printf("Theme: %s\n\n", resolvedCfg.Theme.ThemeName)

	// Create console for replay
	consoleCfg := fo.ConsoleConfig{
		ThemeName:      resolvedCfg.Theme.ThemeName,
		UseBoxes:       resolvedCfg.Theme.Style.UseBoxes,
		UseBoxesSet:    true,
		Monochrome:     resolvedCfg.Theme.IsMonochrome,
		ShowTimer:      false, // No timer for replay
		ShowTimerSet:   true,
		ShowOutputMode: "always",
		PatternHint:    cliFlags.PatternHint,
		Debug:          *debugFlag,
		Design:         resolvedCfg.Theme,
	}

	console := fo.NewConsole(consoleCfg)

	// Create task for processing
	task := design.NewTask(capture.Label, capture.Command, "replay", capture.Args, resolvedCfg.Theme)

	// Process based on show flag
	switch *showOnlyFlag {
	case "stdout":
		if capture.Stdout != "" {
			console.ProcessStdin(task, []byte(capture.Stdout))
		}
	case "stderr":
		if capture.Stderr != "" {
			console.ProcessStdin(task, []byte(capture.Stderr))
		}
	default: // "all"
		// Process stdout first, then stderr
		if capture.Stdout != "" {
			console.ProcessStdin(task, []byte(capture.Stdout))
		}
		if capture.Stderr != "" {
			// Process stderr lines - classify as error/warning based on content
			for _, line := range strings.Split(strings.TrimSuffix(capture.Stderr, "\n"), "\n") {
				if line != "" {
					lineType := "error"
					if strings.Contains(strings.ToLower(line), "warning") ||
						strings.Contains(strings.ToLower(line), "warn") {
						lineType = "warning"
					}
					task.AddOutputLine(line, lineType, design.LineContext{})
				}
			}
		}
	}

	// Render output
	fmt.Println("─── Rendered Output ───")
	for _, line := range task.OutputLines {
		rendered := task.RenderOutputLine(line)
		fmt.Println(rendered)
	}

	return 0
}

// parseGlobalFlagsFromArgs parses global flags from the given args slice using flag.FlagSet.
// This avoids mutating os.Args and allows proper testing.
// Returns (cliFlags, versionFlag, cmdArgs, error) where cmdArgs are the arguments after "--".
func parseGlobalFlagsFromArgs(args []string) (config.CliFlags, bool, []string, error) {
	var cliFlags config.CliFlags
	var versionFlag bool
	var tasksFlag stringSliceFlag

	// Skip program name (args[0]) for flag parsing
	flagArgs := args[1:]

	// Find "--" separator and split args
	var flagsToparse []string
	var cmdArgs []string
	for i, arg := range flagArgs {
		if arg == "--" {
			flagsToparse = flagArgs[:i]
			if i+1 < len(flagArgs) {
				cmdArgs = flagArgs[i+1:]
			}
			break
		}
	}
	if cmdArgs == nil {
		// No "--" found, all args are flags
		flagsToparse = flagArgs
	}

	// Create a FlagSet for global flags
	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	// Define flags for version and help
	fs.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	fs.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")

	// Global flags
	fs.BoolVar(&cliFlags.Debug, "debug", false, "Enable debug output.")
	fs.BoolVar(&cliFlags.Debug, "d", false, "Enable debug output (shorthand).")
	fs.StringVar(&cliFlags.ThemeName, "theme", "", "Select visual theme (e.g., 'ascii_minimal', 'unicode_vibrant').")
	fs.StringVar(&cliFlags.ThemeFile, "theme-file", "", "Load custom theme from YAML file.")
	fs.BoolVar(&cliFlags.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	fs.BoolVar(&cliFlags.CI, "ci", false, "Enable CI-friendly, plain-text output.")
	fs.BoolVar(&cliFlags.Dashboard, "dashboard", false, "Run multiple commands in dashboard mode.")
	fs.Var(&tasksFlag, "task", "Dashboard task in group/name:command format (repeatable).")

	// Flags specific to command wrapping mode
	fs.StringVar(&cliFlags.Label, "l", "", "Label for the task.")
	fs.StringVar(&cliFlags.Label, "label", "", "Label for the task.")
	fs.BoolVar(&cliFlags.LiveStreamOutput, "s", false, "Live stream output mode - print command's stdout/stderr live.")
	fs.BoolVar(&cliFlags.LiveStreamOutput, "stream", false, "Live stream output mode.")
	fs.StringVar(&cliFlags.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never.")
	fs.StringVar(&cliFlags.PatternHint, "pattern", "",
		"Force specific visualization pattern (test-table, sparkline, leaderboard, inventory, summary, comparison).")
	fs.StringVar(&cliFlags.Format, "format", "text",
		"Output format: 'text' (default) or 'json' (structured output for AI/automation).")
	fs.BoolVar(&cliFlags.Profile, "profile", false, "Enable performance profiling.")
	fs.StringVar(&cliFlags.ProfileOutput, "profile-output", "stderr",
		"Profile output destination: 'stderr' (default) or file path.")
	fs.BoolVar(&cliFlags.NoTimer, "no-timer", false, "Disable showing the duration.")

	var maxBufferSizeMB int
	var maxLineLengthKB int
	defaultBufferMB := config.DefaultMaxBufferSize / (1024 * 1024)
	defaultLineKB := config.DefaultMaxLineLength / 1024
	fs.IntVar(&maxBufferSizeMB, "max-buffer-size", 0,
		fmt.Sprintf("Maximum total buffer size in MB (per stream). Default: %dMB", defaultBufferMB))
	fs.IntVar(&maxLineLengthKB, "max-line-length", 0,
		fmt.Sprintf("Maximum length in KB for a single line. Default: %dKB", defaultLineKB))

	if err := fs.Parse(flagsToparse); err != nil {
		return cliFlags, false, nil, err
	}

	cliFlags.Tasks = tasksFlag

	// Track which flags were explicitly set
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "s", "stream":
			cliFlags.LiveStreamOutputSet = true
		case "show-output":
			cliFlags.ShowOutputSet = true
		case "pattern":
			cliFlags.PatternHintSet = true
		case "no-timer":
			cliFlags.NoTimerSet = true
		case "no-color":
			cliFlags.NoColorSet = true
		case "ci":
			cliFlags.CISet = true
		case "d", "debug":
			cliFlags.DebugSet = true
		}
	})

	if maxBufferSizeMB > 0 {
		cliFlags.MaxBufferSize = int64(maxBufferSizeMB) * 1024 * 1024
	}
	if maxLineLengthKB > 0 {
		cliFlags.MaxLineLength = maxLineLengthKB * 1024
	}

	// Validate flag values
	if cliFlags.ShowOutput != "" {
		validValues := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !validValues[cliFlags.ShowOutput] {
			return cliFlags, false, nil, fmt.Errorf(
				"invalid value for --show-output: %s (valid: on-fail, always, never)",
				cliFlags.ShowOutput)
		}
	}

	if cliFlags.Format != "" {
		validFormats := map[string]bool{"text": true, "json": true}
		if !validFormats[cliFlags.Format] {
			return cliFlags, false, nil, fmt.Errorf(
				"invalid value for --format: %s (valid: text, json)",
				cliFlags.Format)
		}
	}

	if cliFlags.PatternHint != "" {
		if !validPatterns[cliFlags.PatternHint] {
			return cliFlags, false, nil, fmt.Errorf(
				"invalid value for --pattern: %s (valid: test-table, sparkline, leaderboard, inventory, summary, comparison)",
				cliFlags.PatternHint)
		}
	}

	return cliFlags, versionFlag, cmdArgs, nil
}

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dkoosis/fo/fo"
	config "github.com/dkoosis/fo/internal/config"
	"github.com/dkoosis/fo/internal/version"
	"github.com/dkoosis/fo/pkg/design"
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

// run executes the application logic and returns the exit code.
// This allows integration tests to invoke the logic without os.Exit() terminating the test runner.
func run(args []string) int {
	// Check for subcommand first
	if len(args) > 1 {
		command := args[1]
		if !strings.HasPrefix(command, "-") { // It's a potential subcommand
			if command == "print" {
				handlePrintCommand(args[2:]) // Pass remaining args to print handler
				return 0
			}
			// Add other subcommands here if needed
		}
	}

	// Temporarily override os.Args for flag parsing
	originalArgs := os.Args
	os.Args = args
	defer func() { os.Args = originalArgs }()

	// If not a recognized subcommand, proceed as command wrapper
	cliFlags, versionFlag := parseGlobalFlags()

	// Handle version flag
	if versionFlag {
		_, _ = fmt.Fprintf(os.Stdout, "fo version %s\n", version.Version)
		_, _ = fmt.Fprintf(os.Stdout, "Commit: %s\n", version.CommitHash)
		_, _ = fmt.Fprintf(os.Stdout, "Built: %s\n", version.BuildDate)
		return 0
	}

	// Load application configuration from .fo.yaml
	fileAppConfig := config.LoadConfig()

	// Find the command and arguments to be executed (must be after "--")
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "[fo] Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "[fo] Usage: fo [flags] -- <COMMAND> [ARGS...]")
		return 1 // Exit if no command is provided
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
		InlineProgress:   finalDesignConfig.Style.UseInlineProgress,
		InlineSet:        true,
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

func findCommandArgs() []string {
	args := os.Args
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

func handlePrintCommand(args []string) {
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
		os.Exit(1)
	}
	messageParts := printFlagSet.Args()
	message := strings.Join(messageParts, " ")

	if message == "" && *typeFlag != "raw" { // Allow empty raw for just printing newline or control chars
		fmt.Fprintln(os.Stderr, "[fo] Error: No message provided for 'fo print'.")
		printFlagSet.Usage()
		os.Exit(1)
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
		os.Exit(1)
	}
	finalDesignConfig := resolvedCfg.Theme

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG handlePrintCommand] Type: %s, Icon: %s, Indent: %d, Message: '%s'\n",
			*typeFlag, *iconFlag, *indentFlag, message)
		fmt.Fprintf(os.Stderr, "[DEBUG handlePrintCommand] finalDesignConfig.ThemeName: %s, IsMonochrome: %t\n",
			finalDesignConfig.ThemeName, finalDesignConfig.IsMonochrome)
	}

	// Use the new render function for direct messages
	output := design.RenderDirectMessage(finalDesignConfig, *typeFlag, *iconFlag, message, *indentFlag)
	_, _ = os.Stdout.WriteString(output) // Print directly to stdout
	os.Exit(0)
}

func parseGlobalFlags() (config.CliFlags, bool) {
	var cliFlags config.CliFlags
	var versionFlag bool

	// Define flags for version and help
	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")

	// These are global flags, also potentially usable by 'print' if implemented
	flag.BoolVar(&cliFlags.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&cliFlags.Debug, "d", false, "Enable debug output (shorthand).")
	flag.StringVar(&cliFlags.ThemeName, "theme", "", "Select visual theme (e.g., 'ascii_minimal', 'unicode_vibrant').")
	flag.StringVar(&cliFlags.ThemeFile, "theme-file", "", "Load custom theme from YAML file.")
	flag.BoolVar(&cliFlags.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flag.BoolVar(&cliFlags.CI, "ci", false, "Enable CI-friendly, plain-text output.")

	// Flags specific to command wrapping mode
	flag.StringVar(&cliFlags.Label, "l", "", "Label for the task.")
	flag.StringVar(&cliFlags.Label, "label", "", "Label for the task.")
	flag.BoolVar(&cliFlags.LiveStreamOutput, "s", false, "Live stream output mode - print command's stdout/stderr live.")
	flag.BoolVar(&cliFlags.LiveStreamOutput, "stream", false, "Live stream output mode.")
	flag.StringVar(&cliFlags.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never.")
	flag.StringVar(&cliFlags.PatternHint, "pattern", "",
		"Force specific visualization pattern (test-table, sparkline, leaderboard, inventory, summary, comparison).")
	flag.StringVar(&cliFlags.Format, "format", "text",
		"Output format: 'text' (default) or 'json' (structured output for AI/automation).")
	flag.BoolVar(&cliFlags.Profile, "profile", false, "Enable performance profiling.")
	flag.StringVar(&cliFlags.ProfileOutput, "profile-output", "stderr",
		"Profile output destination: 'stderr' (default) or file path.")
	flag.BoolVar(&cliFlags.NoTimer, "no-timer", false, "Disable showing the duration.")

	var maxBufferSizeMB int
	var maxLineLengthKB int
	defaultBufferMB := config.DefaultMaxBufferSize / (1024 * 1024)
	defaultLineKB := config.DefaultMaxLineLength / 1024
	flag.IntVar(&maxBufferSizeMB, "max-buffer-size", 0,
		fmt.Sprintf("Maximum total buffer size in MB (per stream). Default: %dMB", defaultBufferMB))
	flag.IntVar(&maxLineLengthKB, "max-line-length", 0,
		fmt.Sprintf("Maximum length in KB for a single line. Default: %dKB", defaultLineKB))

	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
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

	if cliFlags.ShowOutput != "" {
		validValues := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !validValues[cliFlags.ShowOutput] {
			fmt.Fprintf(os.Stderr,
				"[fo] Error: Invalid value for --show-output: %s\n[fo] Valid values are: on-fail, always, never\n",
				cliFlags.ShowOutput)
			flag.Usage()
			os.Exit(1)
		}
	}

	if cliFlags.Format != "" {
		validFormats := map[string]bool{"text": true, "json": true}
		if !validFormats[cliFlags.Format] {
			fmt.Fprintf(os.Stderr, "[fo] Error: Invalid value for --format: %s\n[fo] Valid values are: text, json\n", cliFlags.Format)
			flag.Usage()
			os.Exit(1)
		}
	}

	if cliFlags.PatternHint != "" {
		if !validPatterns[cliFlags.PatternHint] {
			fmt.Fprintf(os.Stderr,
				"[fo] Error: Invalid value for --pattern: %s\n"+
					"[fo] Valid values are: test-table, sparkline, leaderboard, inventory, summary, comparison\n",
				cliFlags.PatternHint)
			flag.Usage()
			os.Exit(1)
		}
	}

	return cliFlags, versionFlag
}

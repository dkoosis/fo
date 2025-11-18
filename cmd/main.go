package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	config "github.com/davidkoosis/fo/internal/config"
	"github.com/davidkoosis/fo/internal/design"
	"github.com/davidkoosis/fo/internal/version"
	"github.com/davidkoosis/fo/mageconsole"
)

// LocalAppConfig holds behavioral settings derived from AppConfig and CLI flags.
type LocalAppConfig struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool // Effective NoTimer after all flags/configs
	NoColor       bool // Effective NoColor (IsMonochrome)
	CI            bool // Effective CI mode
	Debug         bool
	MaxBufferSize int64 // Max total size for combined stdout/stderr in capture mode
	MaxLineLength int   // Max size for a single line from stdout/stderr
}

var (
	versionFlag    bool
	cliFlagsGlobal config.CliFlags // Holds parsed CLI flags
)

// main is the entry point of the application.
func main() {
	// Check for subcommand first
	if len(os.Args) > 1 {
		command := os.Args[1]
		if !strings.HasPrefix(command, "-") { // It's a potential subcommand
			if command == "print" {
				handlePrintCommand(os.Args[2:]) // Pass remaining args to print handler
				return
			}
			// Add other subcommands here if needed
		}
	}

	// If not a recognized subcommand, proceed as command wrapper
	parseGlobalFlags()

	// Handle version flag
	if versionFlag {
		_, _ = fmt.Fprintf(os.Stdout, "fo version %s\n", version.Version)
		_, _ = fmt.Fprintf(os.Stdout, "Commit: %s\n", version.CommitHash)
		_, _ = fmt.Fprintf(os.Stdout, "Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	// Load application configuration from .fo.yaml
	fileAppConfig := config.LoadConfig()

	// Find the command and arguments to be executed (must be after "--")
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1) // Exit if no command is provided
	}

	// Apply any command-specific presets from the config file
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(fileAppConfig, cmdArgs[0])
	}

	// Convert the file-based AppConfig to LocalAppConfig for runtime behavior
	behavioralSettings := convertAppConfigToLocal(fileAppConfig)

	// Override behavioral settings with any explicitly set CLI flags
	if cliFlagsGlobal.Label != "" {
		behavioralSettings.Label = cliFlagsGlobal.Label
	}
	if cliFlagsGlobal.StreamSet {
		behavioralSettings.Stream = cliFlagsGlobal.Stream
	}
	if cliFlagsGlobal.ShowOutputSet && cliFlagsGlobal.ShowOutput != "" {
		behavioralSettings.ShowOutput = cliFlagsGlobal.ShowOutput
	}

	// Debug is ONLY enabled by explicit --debug flag
	if cliFlagsGlobal.DebugSet {
		behavioralSettings.Debug = cliFlagsGlobal.Debug
		fileAppConfig.Debug = cliFlagsGlobal.Debug // Ensure this is passed to MergeWithFlags
	} else {
		// Force debug off unless explicitly enabled by flag
		behavioralSettings.Debug = false
		fileAppConfig.Debug = false
	}

	if cliFlagsGlobal.MaxBufferSize > 0 {
		behavioralSettings.MaxBufferSize = cliFlagsGlobal.MaxBufferSize
	}
	if cliFlagsGlobal.MaxLineLength > 0 {
		behavioralSettings.MaxLineLength = cliFlagsGlobal.MaxLineLength
	}

	// Get the final design configuration (styling, icons, colors) by merging
	// the file configuration with CLI flags related to presentation
	finalDesignConfig := config.MergeWithFlags(fileAppConfig, cliFlagsGlobal)

	// Update behavioralSettings with final decisions on NoTimer, NoColor, CI from finalDesignConfig
	behavioralSettings.NoTimer = finalDesignConfig.Style.NoTimer
	behavioralSettings.NoColor = finalDesignConfig.IsMonochrome
	behavioralSettings.CI = finalDesignConfig.IsMonochrome && finalDesignConfig.Style.NoTimer
	// Ensure debug is still controlled only by explicit --debug flag
	behavioralSettings.Debug = cliFlagsGlobal.DebugSet && cliFlagsGlobal.Debug

	consoleCfg := mageconsole.ConsoleConfig{
		ThemeName:      finalDesignConfig.ThemeName,
		UseBoxes:       finalDesignConfig.Style.UseBoxes,
		UseBoxesSet:    true,
		InlineProgress: finalDesignConfig.Style.UseInlineProgress,
		InlineSet:      true,
		Monochrome:     finalDesignConfig.IsMonochrome,
		ShowTimer:      !finalDesignConfig.Style.NoTimer,
		ShowTimerSet:   true,
		ShowOutputMode: behavioralSettings.ShowOutput,
		Stream:         behavioralSettings.Stream,
		Debug:          behavioralSettings.Debug,
		MaxBufferSize:  behavioralSettings.MaxBufferSize,
		MaxLineLength:  behavioralSettings.MaxLineLength,
		Design:         finalDesignConfig,
	}

	console := mageconsole.NewConsole(consoleCfg)
	result, err := console.Run(behavioralSettings.Label, cmdArgs[0], cmdArgs[1:]...)

	exitCode := 0
	if result != nil {
		exitCode = result.ExitCode
	}
	if err != nil && result == nil {
		exitCode = 1
	}

	if behavioralSettings.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d).\nBehavioral Config: %+v\n", exitCode, behavioralSettings)
	}
	os.Exit(exitCode)
}

func convertAppConfigToLocal(appCfg *config.AppConfig) LocalAppConfig {
	return LocalAppConfig{
		Label:         appCfg.Label,
		Stream:        appCfg.Stream,
		ShowOutput:    appCfg.ShowOutput,
		NoTimer:       appCfg.NoTimer,
		NoColor:       appCfg.NoColor,
		CI:            appCfg.CI,
		Debug:         false, // Default to false, only enable when explicitly set by flag
		MaxBufferSize: appCfg.MaxBufferSize,
		MaxLineLength: appCfg.MaxLineLength,
	}
}

func findCommandArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" {
			if i < len(os.Args)-1 {
				return os.Args[i+1:]
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
		fmt.Fprintf(os.Stderr, "Error parsing 'print' flags: %v\n", err)
		os.Exit(1)
	}
	messageParts := printFlagSet.Args()
	message := strings.Join(messageParts, " ")

	if message == "" && *typeFlag != "raw" { // Allow empty raw for just printing newline or control chars
		fmt.Fprintln(os.Stderr, "Error: No message provided for 'fo print'.")
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

	fileAppConfig := config.LoadConfig() // Load base config
	finalDesignConfig := config.MergeWithFlags(fileAppConfig, globalCliFlagsForPrint)

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

func parseGlobalFlags() {
	// Define flags for version and help
	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")

	// These are global flags, also potentially usable by 'print' if implemented
	flag.BoolVar(&cliFlagsGlobal.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&cliFlagsGlobal.Debug, "d", false, "Enable debug output (shorthand).")
	flag.StringVar(&cliFlagsGlobal.ThemeName, "theme", "", "Select visual theme (e.g., 'ascii_minimal', 'unicode_vibrant').")
	flag.BoolVar(&cliFlagsGlobal.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flag.BoolVar(&cliFlagsGlobal.CI, "ci", false, "Enable CI-friendly, plain-text output.")

	// Flags specific to command wrapping mode
	flag.StringVar(&cliFlagsGlobal.Label, "l", "", "Label for the task.")
	flag.StringVar(&cliFlagsGlobal.Label, "label", "", "Label for the task.")
	flag.BoolVar(&cliFlagsGlobal.Stream, "s", false, "Stream mode - print command's stdout/stderr live.")
	flag.BoolVar(&cliFlagsGlobal.Stream, "stream", false, "Stream mode.")
	flag.StringVar(&cliFlagsGlobal.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never.")
	flag.BoolVar(&cliFlagsGlobal.NoTimer, "no-timer", false, "Disable showing the duration.")

	var maxBufferSizeMB int
	var maxLineLengthKB int
	flag.IntVar(&maxBufferSizeMB, "max-buffer-size", 0, fmt.Sprintf("Maximum total buffer size in MB (per stream). Default: %dMB", config.DefaultMaxBufferSize/(1024*1024)))
	flag.IntVar(&maxLineLengthKB, "max-line-length", 0, fmt.Sprintf("Maximum length in KB for a single line. Default: %dKB", config.DefaultMaxLineLength/1024))

	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "s", "stream":
			cliFlagsGlobal.StreamSet = true
		case "show-output":
			cliFlagsGlobal.ShowOutputSet = true
		case "no-timer":
			cliFlagsGlobal.NoTimerSet = true
		case "no-color":
			cliFlagsGlobal.NoColorSet = true
		case "ci":
			cliFlagsGlobal.CISet = true
		case "d", "debug":
			cliFlagsGlobal.DebugSet = true
		}
	})

	if maxBufferSizeMB > 0 {
		cliFlagsGlobal.MaxBufferSize = int64(maxBufferSizeMB) * 1024 * 1024
	}
	if maxLineLengthKB > 0 {
		cliFlagsGlobal.MaxLineLength = maxLineLengthKB * 1024
	}

	if cliFlagsGlobal.ShowOutput != "" {
		validValues := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !validValues[cliFlagsGlobal.ShowOutput] {
			fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\nValid values are: on-fail, always, never\n", cliFlagsGlobal.ShowOutput)
			flag.Usage()
			os.Exit(1)
		}
	}
}

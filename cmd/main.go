package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	design "github.com/davidkoosis/fo/cmd/internal/config"
	"github.com/davidkoosis/fo/cmd/internal/design"
	"github.com/davidkoosis/fo/cmd/internal/version"
)

// Config holds the command-line options relevant to fo's execution logic.
type Config struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool
	NoColor       bool
	CI            bool
	Debug         bool
	MaxBufferSize int64
	MaxLineLength int
}

// Valid options for --show-output flag.
var validShowOutputValues = map[string]bool{
	"on-fail": true,
	"always":  true,
	"never":   true,
}

// versionFlag is set if the --version or -v flag is passed.
var versionFlag bool
var flagConfig config.CliFlags

func main() {
	// Parse command line flags
	cliFlags := parseFlags()

	// Handle version flag
	if versionFlag {
		fmt.Printf("fo version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	// Load configuration with defaults, file config, and environment variables
	fileConfig := config.LoadConfig()

	// Apply CLI flag overrides
	mergedConfig := config.MergeWithFlags(fileConfig, *cliFlags)

	// Find the command to execute
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1)
	}

	// Apply command-specific preset if available
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(mergedConfig, cmdArgs[0])
	}

	// Set label if provided via flag
	if cliFlags.Label != "" {
		mergedConfig.Label = cliFlags.Label
	}

	// Get the resolved design configuration for rendering
	designConfig := mergedConfig.GetResolvedDesignConfig()

	// Setup signal handling and context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Execute the command with the resolved configuration
	exitCode := executeCommand(ctx, cancel, sigChan, mergedConfig, designConfig, cmdArgs)

	// Debug output if enabled
	if mergedConfig.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d). Config: %+v\n", exitCode, mergedConfig)
	}

	os.Exit(exitCode)
}

func convertToInternalConfig(cfg *config.Config) Config {
	return Config{
		Label:         cfg.Label,
		Stream:        cfg.Stream,
		ShowOutput:    cfg.ShowOutput,
		NoTimer:       cfg.NoTimer,
		NoColor:       cfg.NoColor,
		CI:            cfg.CI,
		Debug:         cfg.Debug,
		MaxBufferSize: cfg.MaxBufferSize,
		MaxLineLength: cfg.MaxLineLength,
	}
}

func parseFlags() *config.CliFlags {
	// Version flags
	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")

	// Debug flags
	flag.BoolVar(&flagConfig.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&flagConfig.Debug, "d", false, "Enable debug output (shorthand).")
	flagConfig.DebugSet = true // Record that debug flag was explicitly handled

	// Label flag
	flag.StringVar(&flagConfig.Label, "l", "", "Label for the task.")
	flag.StringVar(&flagConfig.Label, "label", "", "Label for the task (shorthand: -l).")

	// Stream mode flag
	flag.BoolVar(&flagConfig.Stream, "s", false, "Stream mode - print command's stdout/stderr live.")
	flag.BoolVar(&flagConfig.Stream, "stream", false, "Stream mode - print command's stdout/stderr live (shorthand: -s).")
	flagConfig.StreamSet = true // Record that stream flag was explicitly handled

	// Output mode flag
	flag.StringVar(&flagConfig.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never.")
	flagConfig.ShowOutputSet = (flagConfig.ShowOutput != "") // Record if this flag was set

	// Timer flag
	flag.BoolVar(&flagConfig.NoTimer, "no-timer", false, "Disable showing the duration.")
	flagConfig.NoTimerSet = true // Record that this flag was explicitly handled

	// Color flag
	flag.BoolVar(&flagConfig.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flagConfig.NoColorSet = true // Record that this flag was explicitly handled

	// CI mode flag
	flag.BoolVar(&flagConfig.CI, "ci", false, "Enable CI-friendly, plain-text output.")
	flagConfig.CISet = true // Record that this flag was explicitly handled

	// Theme selection flag - NEW
	flag.StringVar(&flagConfig.ThemeName, "theme", "", "Select visual theme (e.g., 'ascii_minimal', 'unicode_vibrant').")

	// Buffer size flags
	var maxBufferSizeMB int
	var maxLineLengthKB int
	flag.IntVar(&maxBufferSizeMB, "max-buffer-size", 0,
		fmt.Sprintf("Maximum total buffer size in MB (per stream). Default: %dMB", config.DefaultMaxBufferSize/(1024*1024)))
	flag.IntVar(&maxLineLengthKB, "max-line-length", 0,
		fmt.Sprintf("Maximum length in KB for a single line. Default: %dKB", config.DefaultMaxLineLength/1024))

	flag.Parse()

	// Convert buffer sizes from MB/KB to bytes
	if maxBufferSizeMB > 0 {
		flagConfig.MaxBufferSize = int64(maxBufferSizeMB) * 1024 * 1024
	}
	if maxLineLengthKB > 0 {
		flagConfig.MaxLineLength = maxLineLengthKB * 1024
	}

	// Validate show-output flag value
	if flagConfig.ShowOutput != "" {
		if flagConfig.ShowOutput != "on-fail" &&
			flagConfig.ShowOutput != "always" &&
			flagConfig.ShowOutput != "never" {
			fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\n", flagConfig.ShowOutput)
			fmt.Fprintln(os.Stderr, "Valid values are: on-fail, always, never")
			os.Exit(1)
		}
	}

	return &flagConfig
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

func executeCommand(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal,
	appConfig *config.Config, designConfig *design.Config, cmdArgs []string) int {

	// Determine label (command name if not specified)
	label := appConfig.Label
	if label == "" {
		label = filepath.Base(cmdArgs[0])
	}

	// Create a pattern matcher for output classification
	patternMatcher := design.NewPatternMatcher(designConfig)

	// Detect command intent
	intent := patternMatcher.DetectCommandIntent(cmdArgs[0], cmdArgs[1:])

	// Create task with the resolved design configuration
	task := design.NewTask(label, intent, cmdArgs[0], cmdArgs[1:], designConfig)

	// Show start message
	fmt.Println(task.RenderStartLine())

	// Command setup
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Signal handling goroutine
	cmdDone := make(chan struct{})
	go func() {
		defer close(cmdDone)
		select {
		case sig := <-sigChan:
			if cmd.Process == nil {
				cancel()
				return
			}
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				_ = syscall.Kill(-pgid, sig.(syscall.Signal))
			} else {
				_ = cmd.Process.Signal(sig)
			}
			select {
			case <-cmdDone:
			case <-time.After(2 * time.Second):
				if cmd.Process != nil && cmd.ProcessState == nil {
					pgidKill, errKill := syscall.Getpgid(cmd.Process.Pid)
					if errKill == nil {
						_ = syscall.Kill(-pgidKill, syscall.SIGKILL)
					} else {
						_ = cmd.Process.Kill()
					}
				}
				cancel()
			}
			return
		case <-ctx.Done():
			if cmd.Process != nil && cmd.ProcessState == nil {
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				} else {
					_ = cmd.Process.Kill()
				}
			}
			return
		}
	}()

	// Execute in stream or capture mode based on config
	var exitCode int
	if appConfig.Stream {
		exitCode = executeStreamMode(cmd, task, patternMatcher)
	} else {
		exitCode = executeCaptureMode(cmd, task, patternMatcher, appConfig)
	}

	// Complete the task with the exit code
	task.Complete(exitCode)

	// Printing logic for captured output and summary
	if !appConfig.Stream {
		if appConfig.ShowOutput == "on-fail" && exitCode != 0 {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
			for _, line := range task.OutputLines {
				fmt.Println(task.RenderOutputLine(line))
			}
		} else if appConfig.ShowOutput == "always" {
			// Lines were already printed by processOutputPipe.
			// Print summary if there were issues (errors or warnings).
			if task.Status == design.StatusError || task.Status == design.StatusWarning {
				summary := task.RenderSummary()
				if summary != "" {
					fmt.Print(summary)
				}
			}
		}
	} else { // Stream mode
		// If stream mode resulted in an error or warning status, print summary.
		if task.Status == design.StatusError || task.Status == design.StatusWarning {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	}

	// Show end message
	fmt.Println(task.RenderEndLine())
	return exitCode
}

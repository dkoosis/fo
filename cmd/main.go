package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	// Corrected imports
	config "github.com/davidkoosis/fo/cmd/internal/config" // Use 'config' as the package name
	"github.com/davidkoosis/fo/cmd/internal/design"        // This is the design package
	"github.com/davidkoosis/fo/cmd/internal/version"
)

// Local Config struct for behavioral settings passed to executeCommand.
// This acts as a bridge/model for the settings that control fo's execution logic,
// distinct from design.Config which controls styling.
type LocalAppConfig struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool // This might be duplicative if design.Config also has it and is source of truth
	NoColor       bool // Same as NoTimer
	CI            bool // Same as NoTimer
	Debug         bool
	MaxBufferSize int64
	MaxLineLength int
}

// versionFlag is set if the --version or -v flag is passed.
var versionFlag bool

// Global var to hold parsed CLI flags. parseFlags will populate this.
var cliFlagsGlobal config.CliFlags

func main() {
	// Parse command line flags, populates cliFlagsGlobal
	parseFlagsIntoGlobal()

	if versionFlag {
		fmt.Printf("fo version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	// 1. Load base application configuration (from .fo.yaml, includes themes, presets, global defaults)
	fileAppConfig := config.LoadConfig() // Returns *config.AppConfig

	// 2. Apply command-specific presets to the fileAppConfig if any match
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		// This check should ideally be after flag parsing, including help flags.
		// For now, keeping it simple as per original structure.
		// Check if help was requested by flags; if so, print usage and exit.
		// flag.Usage() can be customized.
		// Example: if any help flag is set, call flag.Usage() and os.Exit(0)
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1) // Exit if no command is provided
	}
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(fileAppConfig, cmdArgs[0]) // Modifies fileAppConfig based on presets
	}

	// 3. Create the behavioral configuration (LocalAppConfig)
	// Start with defaults from fileAppConfig (which includes preset modifications)
	behavioralSettings := convertAppConfigToLocal(fileAppConfig)

	// Override behavioral settings with CLI flags (CLI has highest precedence for behavior)
	if cliFlagsGlobal.Label != "" {
		behavioralSettings.Label = cliFlagsGlobal.Label
	}
	if cliFlagsGlobal.StreamSet {
		behavioralSettings.Stream = cliFlagsGlobal.Stream
	}
	if cliFlagsGlobal.ShowOutputSet && cliFlagsGlobal.ShowOutput != "" {
		behavioralSettings.ShowOutput = cliFlagsGlobal.ShowOutput
	}
	if cliFlagsGlobal.DebugSet { // Debug is primarily an app-level behavioral flag
		behavioralSettings.Debug = cliFlagsGlobal.Debug
		fileAppConfig.Debug = cliFlagsGlobal.Debug // Ensure AppConfig also reflects CLI debug for MergeWithFlags
	}
	if cliFlagsGlobal.MaxBufferSize > 0 { // Assuming 0 means not set by flag
		behavioralSettings.MaxBufferSize = cliFlagsGlobal.MaxBufferSize
	}
	if cliFlagsGlobal.MaxLineLength > 0 { // Assuming 0 means not set by flag
		behavioralSettings.MaxLineLength = cliFlagsGlobal.MaxLineLength
	}
	// NoTimer, NoColor, CI from cliFlagsGlobal will be handled by MergeWithFlags for design.Config

	// 4. Get the final design configuration (for styling)
	// config.MergeWithFlags takes the *config.AppConfig (which has presets applied and debug flag updated)
	// and *config.CliFlags to produce the final *design.Config.
	finalDesignConfig := config.MergeWithFlags(fileAppConfig, cliFlagsGlobal)

	// Setup signal handling and context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Execute the command
	// Pass the derived behavioralSettings and the finalDesignConfig
	exitCode := executeCommand(ctx, cancel, sigChan, behavioralSettings, finalDesignConfig, cmdArgs)

	if behavioralSettings.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d).\nBehavioral Config: %+v\n", exitCode, behavioralSettings)
		// Note: finalDesignConfig can be very large to print.
	}

	os.Exit(exitCode)
}

// convertAppConfigToLocal converts from the loaded *config.AppConfig
// to the LocalAppConfig struct used for behavioral settings in executeCommand.
func convertAppConfigToLocal(appCfg *config.AppConfig) LocalAppConfig {
	return LocalAppConfig{
		Label:         appCfg.Label, // Label from AppConfig (after presets) becomes base for behavioral
		Stream:        appCfg.Stream,
		ShowOutput:    appCfg.ShowOutput,
		NoTimer:       appCfg.NoTimer, // Base value from AppConfig
		NoColor:       appCfg.NoColor, // Base value from AppConfig
		CI:            appCfg.CI,      // Base value from AppConfig
		Debug:         appCfg.Debug,
		MaxBufferSize: appCfg.MaxBufferSize,
		MaxLineLength: appCfg.MaxLineLength,
	}
}

// parseFlagsIntoGlobal parses command line flags and populates the global cliFlagsGlobal variable.
func parseFlagsIntoGlobal() {
	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")

	// Use cliFlagsGlobal directly
	flag.BoolVar(&cliFlagsGlobal.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&cliFlagsGlobal.Debug, "d", false, "Enable debug output (shorthand).")
	// Record if flag was set (important for boolean flags where default false is ambiguous)
	// This requires checking if the flag was actually present on the command line.
	// A common way is to check flag.Visit after flag.Parse().
	// For simplicity here, we'll use the XxxSet booleans in CliFlags.

	flag.StringVar(&cliFlagsGlobal.Label, "l", "", "Label for the task.")
	flag.StringVar(&cliFlagsGlobal.Label, "label", "", "Label for the task.")

	flag.BoolVar(&cliFlagsGlobal.Stream, "s", false, "Stream mode - print command's stdout/stderr live.")
	flag.BoolVar(&cliFlagsGlobal.Stream, "stream", false, "Stream mode.")

	flag.StringVar(&cliFlagsGlobal.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never.")

	flag.BoolVar(&cliFlagsGlobal.NoTimer, "no-timer", false, "Disable showing the duration.")
	flag.BoolVar(&cliFlagsGlobal.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flag.BoolVar(&cliFlagsGlobal.CI, "ci", false, "Enable CI-friendly, plain-text output.")
	flag.StringVar(&cliFlagsGlobal.ThemeName, "theme", "", "Select visual theme (e.g., 'ascii_minimal', 'unicode_vibrant').")

	var maxBufferSizeMB int
	var maxLineLengthKB int
	flag.IntVar(&maxBufferSizeMB, "max-buffer-size", 0,
		fmt.Sprintf("Maximum total buffer size in MB (per stream). Default: %dMB", config.DefaultMaxBufferSize/(1024*1024)))
	flag.IntVar(&maxLineLengthKB, "max-line-length", 0,
		fmt.Sprintf("Maximum length in KB for a single line. Default: %dKB", config.DefaultMaxLineLength/1024))

	flag.Parse() // Parse all defined flags

	// After parsing, update the XxxSet fields
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
		validShowOutputValues := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !validShowOutputValues[cliFlagsGlobal.ShowOutput] {
			fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\n", cliFlagsGlobal.ShowOutput)
			fmt.Fprintln(os.Stderr, "Valid values are: on-fail, always, never")
			flag.Usage() // Print default usage information
			os.Exit(1)
		}
	}
}

func findCommandArgs() []string {
	args := flag.Args() // Unparsed arguments after flags
	if len(args) > 0 && args[0] == "--" {
		if len(args) > 1 {
			return args[1:]
		}
		return []string{} // Only "--" was found
	}
	// If "--" is not used, all non-flag args are considered the command.
	// However, the spec says "fo [flags] -- <COMMAND>".
	// For strictness, require "--". If not found, and flag.Args() is not empty,
	// it might be an error or an attempt to run fo without "--".
	// The original findCommandArgs iterated os.Args. flag.Args() is usually preferred.
	// Reverting to original logic for "--" separator:
	for i, arg := range os.Args {
		if arg == "--" {
			if i < len(os.Args)-1 {
				return os.Args[i+1:]
			}
			return []string{} // "--" was the last argument
		}
	}
	// If "--" was not found, it implies an error or old usage pattern.
	// Depending on strictness, could return flag.Args() or error.
	// For now, if "--" is missing, no command is considered passed this way.
	return []string{}
}

// executeCommand now takes LocalAppConfig for behavioral settings and *design.Config for styling.
func executeCommand(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal,
	appSettings LocalAppConfig, designCfg *design.Config, cmdArgs []string) int {

	labelToUse := appSettings.Label // Label from CLI or preset or .fo.yaml global
	if labelToUse == "" {
		labelToUse = filepath.Base(cmdArgs[0]) // Default to command name
	}

	patternMatcher := design.NewPatternMatcher(designCfg) // Pattern matcher uses design config
	intent := patternMatcher.DetectCommandIntent(cmdArgs[0], cmdArgs[1:])
	task := design.NewTask(labelToUse, intent, cmdArgs[0], cmdArgs[1:], designCfg)

	fmt.Println(task.RenderStartLine()) // Uses designCfg

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmdDone := make(chan struct{})
	go func() {
		defer close(cmdDone)
		select {
		case sig := <-sigChan:
			if cmd.Process == nil {
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Process is nil, canceling context.")
				}
				cancel() // Cancel context if process hasn't started
				return
			}
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Received signal %v for PID %d. Forwarding...\n", sig, cmd.Process.Pid)
			}
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				if appSettings.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Sending signal %v to PGID %d\n", sig, pgid)
				}
				_ = syscall.Kill(-pgid, sig.(syscall.Signal)) // Send to whole process group
			} else {
				if appSettings.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Failed to get PGID for PID %d (%v), sending to PID directly.\n", cmd.Process.Pid, err)
				}
				_ = cmd.Process.Signal(sig) // Fallback to sending to process itself
			}
			// Wait for command to exit or timeout for cleanup
			select {
			case <-cmdDone:
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received after signal forwarding.")
				}
			case <-time.After(2 * time.Second): // Grace period for termination
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Timeout after signal, ensuring process is killed.")
				}
				if cmd.Process != nil && cmd.ProcessState == nil { // If still running
					pgidKill, errKill := syscall.Getpgid(cmd.Process.Pid)
					if errKill == nil {
						_ = syscall.Kill(-pgidKill, syscall.SIGKILL) // Force kill group
					} else {
						_ = cmd.Process.Kill() // Force kill process
					}
				}
				cancel() // Cancel context as a final measure
			}
			return
		case <-ctx.Done(): // Context was cancelled (e.g., by timeout or other logic)
			if appSettings.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Context done, ensuring process is killed if running.")
			}
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

	var exitCode int
	// Behavioral flags from appSettings control execution mode
	if appSettings.Stream {
		// executeStreamMode now needs appSettings (LocalAppConfig) for MaxBufferSize/LineLength
		// and designCfg for patternMatcher and task methods if they use it.
		// Let's assume executeStreamMode primarily uses task for output, which has designCfg.
		// It might also need behavioral flags directly.
		exitCode = executeStreamMode(cmd, task, patternMatcher /*, appSettings needed if it uses MaxBuffer/LineLength */)
	} else {
		// executeCaptureMode needs appSettings for MaxBufferSize/LineLength etc.
		exitCode = executeCaptureMode(cmd, task, patternMatcher, appSettings)
	}

	task.Complete(exitCode) // Uses designCfg for status determination

	// Printing logic based on behavioral ShowOutput flag
	if !appSettings.Stream {
		showCaptured := false
		switch appSettings.ShowOutput {
		case "always":
			showCaptured = true
		case "on-fail":
			if exitCode != 0 {
				showCaptured = true
			}
		case "never":
			showCaptured = false
		}

		if showCaptured {
			summary := task.RenderSummary() // Uses designCfg
			if summary != "" {
				fmt.Print(summary)
			}
			// Print header for captured output only if showing output
			fmt.Println(designCfg.GetColor("Muted"), "--- Captured output: ---", designCfg.ResetColor())

			for _, line := range task.OutputLines {
				fmt.Println(task.RenderOutputLine(line)) // Uses designCfg
			}
		} else if task.Status == design.StatusError || task.Status == design.StatusWarning {
			// Even if not showing full output, show summary if there were issues
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	} else { // Stream mode
		if task.Status == design.StatusError || task.Status == design.StatusWarning {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	}

	fmt.Println(task.RenderEndLine()) // Uses designCfg
	return exitCode
}
func executeStreamMode(cmd *exec.Cmd, task *design.Task) int {
	// In stream mode, we want to pipe stdout directly.
	// For stderr, we can tee it: one to os.Stderr for live view, one to a buffer for task.OutputLines.
	// This allows Task.Complete to correctly assess status based on stderr content if needed.

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		// Cannot create stderr pipe, fallback to direct os.Stderr and lose capture for summary
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr := cmd.Run()
		// Add a generic error to task if pipe creation failed, as we can't capture details
		task.AddOutputLine(fmt.Sprintf("Error setting up stderr pipe for stream mode: %v", err), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		return getExitCode(runErr)
	}
	cmd.Stdout = os.Stdout // Stdout goes directly to os.Stdout

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Tee stderr: print to os.Stderr and also add to task.OutputLines
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line) // Print to actual stderr
			// Use a simple classification for streamed stderr, or enhance if needed
			// For now, just add as TypeInfo to allow summary generation
			task.AddOutputLine(line, design.TypeInfo, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 2})
		}
		if err := scanner.Err(); err != nil {
			// Log scanner error for stderr if necessary, but don't let it change main exit code path
			// This error is about reading the pipe, not the command's exit status.
			// Add it to the task for potential summary.
			task.AddOutputLine(fmt.Sprintf("Error reading stderr in stream mode: %v", err), design.TypeError, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 3})
		}
	}()

	runErr := cmd.Run() // This runs the command and waits for it.
	wg.Wait()           // Wait for stderr processing to complete.

	return getExitCode(runErr)
}

func executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, appConfig Config) int {
	var wg sync.WaitGroup
	var bufferExceeded sync.Once

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating stdout pipe: %v", err)
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5}
		task.AddOutputLine(errMsg, design.TypeError, errCtx)
		if appConfig.ShowOutput == "always" || appConfig.ShowOutput == "on-fail" {
			fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
		}
		return 1
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating stderr pipe: %v", err)
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5}
		task.AddOutputLine(errMsg, design.TypeError, errCtx)
		if appConfig.ShowOutput == "always" || appConfig.ShowOutput == "on-fail" {
			fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
		}
		return 1
	}

	processOutputPipe := func(pipe io.ReadCloser, source string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), appConfig.MaxLineLength)

		var currentTotalBytes int64
		for scanner.Scan() {
			line := scanner.Text()
			lineLength := int64(len(line))

			if currentTotalBytes+lineLength > appConfig.MaxBufferSize {
				bufferExceeded.Do(func() {
					exceededMsg := fmt.Sprintf("[fo] BUFFER LIMIT: %s stream exceeded %dMB. Further output truncated.", source, appConfig.MaxBufferSize/(1024*1024))
					warnCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4}
					task.AddOutputLine(exceededMsg, design.TypeWarning, warnCtx)
					if appConfig.ShowOutput == "always" {
						fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
					}
				})
				break
			}
			currentTotalBytes += lineLength

			lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
			// If pattern matcher classifies as TypeDetail (default for unrecognised lines) AND source is stderr,
			// then re-classify as TypeInfo. This prevents generic stderr from becoming TypeError via hasOutputIssues.
			if source == "stderr" && lineType == design.TypeDetail {
				lineType = design.TypeInfo // Default unclassified stderr to Info
				lineContext.Importance = 3 // Adjust importance for info
			}
			task.AddOutputLine(line, lineType, lineContext)
			task.UpdateTaskContext()

			if appConfig.ShowOutput == "always" {
				fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
			}
		}

		if errScan := scanner.Err(); errScan != nil {
			if errors.Is(errScan, bufio.ErrTooLong) {
				bufferExceeded.Do(func() {
					exceededMsg := fmt.Sprintf("[fo] LINE LIMIT: Max line length (%d KB) exceeded in %s. Line truncated.", appConfig.MaxLineLength/1024, source)
					warnCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4}
					task.AddOutputLine(exceededMsg, design.TypeWarning, warnCtx)
					if appConfig.ShowOutput == "always" {
						fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
					}
				})
			} else if !strings.Contains(errScan.Error(), "file already closed") && !strings.Contains(errScan.Error(), "broken pipe") {
				errMsg := fmt.Sprintf("Error reading %s: %v", source, errScan)
				errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4}
				task.AddOutputLine(errMsg, design.TypeError, errCtx)
				if appConfig.ShowOutput == "always" {
					fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
				}
			}
		}
	}

	wg.Add(2)
	go processOutputPipe(stdoutPipe, "stdout")
	go processOutputPipe(stderrPipe, "stderr")

	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5}
		task.AddOutputLine(errMsg, design.TypeError, errCtx)
		fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
		fmt.Fprintln(os.Stderr, errMsg)
		return getExitCode(err)
	}

	cmdErr := cmd.Wait()
	wg.Wait()

	return getExitCode(cmdErr)
}

// executeStreamMode and executeCaptureMode need to be defined in a separate file
// or later in this file. For now, I'll assume they exist and have signatures like:
// func executeStreamMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher) int
// func executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, appSettings LocalAppConfig) int
// You will need to adjust them if their current implementations in your project are different
// or if they need more/different parameters. The key is that `appSettings` provides behavioral
// config and `task.Config` (which is `designCfg`) provides styling config.

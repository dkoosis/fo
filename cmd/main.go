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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	config "github.com/davidkoosis/fo/cmd/internal/config"
	"github.com/davidkoosis/fo/cmd/internal/design"
	"github.com/davidkoosis/fo/cmd/internal/version"
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

var versionFlag bool
var cliFlagsGlobal config.CliFlags // Holds parsed CLI flags

// main is the entry point of the application.
func main() {
	// Parse command-line flags into the global cliFlagsGlobal struct.
	parseFlagsIntoGlobal()

	// Handle version flag.
	if versionFlag {
		fmt.Printf("fo version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	// Load application configuration from .fo.yaml.
	fileAppConfig := config.LoadConfig()

	// Find the command and arguments to be executed (must be after "--").
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1) // Exit if no command is provided.
	}

	// Apply any command-specific presets from the config file.
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(fileAppConfig, cmdArgs[0])
	}

	// Convert the file-based AppConfig to LocalAppConfig for runtime behavior.
	behavioralSettings := convertAppConfigToLocal(fileAppConfig)

	// Override behavioral settings with any explicitly set CLI flags.
	if cliFlagsGlobal.Label != "" {
		behavioralSettings.Label = cliFlagsGlobal.Label
	}
	if cliFlagsGlobal.StreamSet { // Check if -s/--stream was actually used
		behavioralSettings.Stream = cliFlagsGlobal.Stream
	}
	if cliFlagsGlobal.ShowOutputSet && cliFlagsGlobal.ShowOutput != "" { // Check if --show-output was used
		behavioralSettings.ShowOutput = cliFlagsGlobal.ShowOutput
	}
	if cliFlagsGlobal.DebugSet { // CLI debug flag overrides all
		behavioralSettings.Debug = cliFlagsGlobal.Debug
		fileAppConfig.Debug = cliFlagsGlobal.Debug // Ensure this is passed to MergeWithFlags
	}
	if cliFlagsGlobal.MaxBufferSize > 0 { // CLI overrides config if set
		behavioralSettings.MaxBufferSize = cliFlagsGlobal.MaxBufferSize
	}
	if cliFlagsGlobal.MaxLineLength > 0 { // CLI overrides config if set
		behavioralSettings.MaxLineLength = cliFlagsGlobal.MaxLineLength
	}

	// Get the final design configuration (styling, icons, colors) by merging
	// the file configuration with CLI flags related to presentation.
	finalDesignConfig := config.MergeWithFlags(fileAppConfig, cliFlagsGlobal)

	// Update behavioralSettings with final decisions on NoTimer, NoColor, CI from finalDesignConfig.
	// This ensures consistency if executeCommand logic depends on these.
	behavioralSettings.NoTimer = finalDesignConfig.Style.NoTimer
	behavioralSettings.NoColor = finalDesignConfig.IsMonochrome
	// CI mode implies no color and no timer for simplicity.
	behavioralSettings.CI = finalDesignConfig.IsMonochrome && finalDesignConfig.Style.NoTimer
	// Ensure behavioralSettings.Debug reflects the final decision (CLI > file config)
	// This is important because executeCommand uses behavioralSettings.Debug
	if fileAppConfig.Debug { // If debug was true in file config
		behavioralSettings.Debug = true
	}
	if cliFlagsGlobal.DebugSet { // And CLI overrides it
		behavioralSettings.Debug = cliFlagsGlobal.Debug
	}

	// Set up context for cancellation and signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is cancelled on exit.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Stop listening for signals on exit.

	// Execute the command and get its exit code.
	exitCode := executeCommand(ctx, cancel, sigChan, behavioralSettings, finalDesignConfig, cmdArgs)

	// Optional debug output before exiting.
	if behavioralSettings.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d).\nBehavioral Config: %+v\n", exitCode, behavioralSettings)
	}
	os.Exit(exitCode) // Exit with the command's exit code.
}

// convertAppConfigToLocal translates settings from the YAML-loaded AppConfig
// to the LocalAppConfig struct used for direct operational control.
func convertAppConfigToLocal(appCfg *config.AppConfig) LocalAppConfig {
	return LocalAppConfig{
		Label:         appCfg.Label,
		Stream:        appCfg.Stream,
		ShowOutput:    appCfg.ShowOutput,
		NoTimer:       appCfg.NoTimer,
		NoColor:       appCfg.NoColor,
		CI:            appCfg.CI,
		Debug:         appCfg.Debug,
		MaxBufferSize: appCfg.MaxBufferSize,
		MaxLineLength: appCfg.MaxLineLength,
	}
}

// parseFlagsIntoGlobal parses all command-line flags and stores them
// in the global cliFlagsGlobal variable. It also handles validation for --show-output.
func parseFlagsIntoGlobal() {
	// Define all CLI flags.
	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")
	flag.BoolVar(&cliFlagsGlobal.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&cliFlagsGlobal.Debug, "d", false, "Enable debug output (shorthand).")
	flag.StringVar(&cliFlagsGlobal.Label, "l", "", "Label for the task.")
	flag.StringVar(&cliFlagsGlobal.Label, "label", "", "Label for the task.")
	flag.BoolVar(&cliFlagsGlobal.Stream, "s", false, "Stream mode - print command's stdout/stderr live.")
	flag.BoolVar(&cliFlagsGlobal.Stream, "stream", false, "Stream mode.")
	flag.StringVar(&cliFlagsGlobal.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never.")
	flag.BoolVar(&cliFlagsGlobal.NoTimer, "no-timer", false, "Disable showing the duration.")
	flag.BoolVar(&cliFlagsGlobal.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flag.BoolVar(&cliFlagsGlobal.CI, "ci", false, "Enable CI-friendly, plain-text output.")
	flag.StringVar(&cliFlagsGlobal.ThemeName, "theme", "", "Select visual theme (e.g., 'ascii_minimal', 'unicode_vibrant').")

	var maxBufferSizeMB int // For user-friendly MB input
	var maxLineLengthKB int // For user-friendly KB input
	flag.IntVar(&maxBufferSizeMB, "max-buffer-size", 0, fmt.Sprintf("Maximum total buffer size in MB (per stream). Default: %dMB", config.DefaultMaxBufferSize/(1024*1024)))
	flag.IntVar(&maxLineLengthKB, "max-line-length", 0, fmt.Sprintf("Maximum length in KB for a single line. Default: %dKB", config.DefaultMaxLineLength/1024))

	flag.Parse() // Parse the flags.

	// Track which flags were explicitly set by the user.
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

	// Convert MB/KB inputs to bytes.
	if maxBufferSizeMB > 0 {
		cliFlagsGlobal.MaxBufferSize = int64(maxBufferSizeMB) * 1024 * 1024
	}
	if maxLineLengthKB > 0 {
		cliFlagsGlobal.MaxLineLength = maxLineLengthKB * 1024
	}

	// Validate the --show-output flag value if it was set.
	if cliFlagsGlobal.ShowOutput != "" {
		validValues := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !validValues[cliFlagsGlobal.ShowOutput] {
			fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\nValid values are: on-fail, always, never\n", cliFlagsGlobal.ShowOutput)
			flag.Usage() // Print usage information.
			os.Exit(1)   // Exit due to invalid flag.
		}
	}
}

// findCommandArgs isolates the command and its arguments from os.Args.
// The command is expected to appear after a "--" separator.
func findCommandArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" {
			// If "--" is found, the subsequent arguments are the command and its args.
			if i < len(os.Args)-1 {
				return os.Args[i+1:]
			}
			return []string{} // "--" was the last argument, no command provided.
		}
	}
	// If "--" is not found, no command is considered specified by this convention.
	return []string{}
}

// executeCommand manages the execution of the wrapped command, including output handling and signal propagation.
func executeCommand(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal,
	appSettings LocalAppConfig, designCfg *design.Config, cmdArgs []string) int {

	// Determine the label for the task. Use CLI flag, then config, then infer from command.
	labelToUse := appSettings.Label // Already incorporates config and CLI override
	if labelToUse == "" {
		labelToUse = filepath.Base(cmdArgs[0]) // Default to the base name of the command.
	}

	// Initialize pattern matcher and task for design system.
	patternMatcher := design.NewPatternMatcher(designCfg)
	intent := patternMatcher.DetectCommandIntent(cmdArgs[0], cmdArgs[1:])
	task := design.NewTask(labelToUse, intent, cmdArgs[0], cmdArgs[1:], designCfg)

	fmt.Println(task.RenderStartLine()) // Print the task start line.

	// Prepare the command for execution with context for cancellation.
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = os.Environ()                                // Inherit environment variables.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Create new process group for signal handling.

	cmdDone := make(chan struct{}) // Channel to signal command completion.
	// Goroutine to handle signals (SIGINT, SIGTERM) and context cancellation.
	go func() {
		defer close(cmdDone) // Ensure cmdDone is closed when this goroutine exits.
		select {
		case sig := <-sigChan: // Received an OS signal.
			if cmd.Process == nil { // Command hasn't started or already finished.
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Process is nil, canceling context.")
				}
				cancel() // Cancel the context.
				return
			}
			// Forward the signal to the entire process group of the command.
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Received signal %v for PID %d. Forwarding...\n", sig, cmd.Process.Pid)
			}
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				if appSettings.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Sending signal %v to PGID %d\n", sig, pgid)
				}
				_ = syscall.Kill(-pgid, sig.(syscall.Signal)) // Negative PGID signals the group.
			} else { // Fallback if getting PGID fails.
				if appSettings.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Failed to get PGID for PID %d (%v), sending to PID directly.\n", cmd.Process.Pid, err)
				}
				_ = cmd.Process.Signal(sig)
			}
			// Wait for the command to react to the signal or timeout.
			select {
			case <-cmdDone: // Command completed after signal.
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received after signal forwarding.")
				}
			case <-time.After(2 * time.Second): // Grace period for command to exit.
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Timeout after signal, ensuring process is killed.")
				}
				// Force kill if still running.
				if cmd.Process != nil && cmd.ProcessState == nil {
					pgidKill, errKill := syscall.Getpgid(cmd.Process.Pid)
					if errKill == nil {
						_ = syscall.Kill(-pgidKill, syscall.SIGKILL)
					} else {
						_ = cmd.Process.Kill()
					}
				}
				cancel() // Cancel context as a final measure.
			}
			return
		case <-ctx.Done(): // Context was cancelled (e.g., by another part of fo or timeout).
			if appSettings.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Context done, ensuring process is killed if running.")
			}
			// Ensure process is killed if context is cancelled.
			if cmd.Process != nil && cmd.ProcessState == nil {
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				} else {
					_ = cmd.Process.Kill()
				}
			}
			return
		case <-cmdDone: // Command finished naturally before any signal or context cancellation.
			if appSettings.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received, command finished naturally.")
			}
			return
		}
	}()

	var exitCode int
	var cmdRunError error             // Stores error from cmd.Start(), cmd.Wait(), or cmd.Run().
	var isActualFoStartupFailure bool // True if fo itself failed to start/pipe the command.

	// Execute in stream or capture mode based on settings.
	if appSettings.Stream {
		exitCode, cmdRunError = executeStreamMode(cmd, task, appSettings)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			// If the error is not an *exec.ExitError, it's likely a startup/pipe issue for fo.
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	} else { // Capture Mode
		exitCode, cmdRunError = executeCaptureMode(cmd, task, patternMatcher, appSettings)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			// If the error is not an *exec.ExitError, it's likely a startup/pipe issue for fo.
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	}
	// cmdDone is closed by the signal handling goroutine's defer statement.

	task.Complete(exitCode) // Mark the task as complete and set its final status.

	// Handle display of captured output for non-stream mode.
	if !appSettings.Stream {
		showCaptured := false
		switch appSettings.ShowOutput {
		case "always":
			showCaptured = true
		case "on-fail":
			if exitCode != 0 { // Show if the command failed.
				showCaptured = true
			}
			// "never" remains false.
		}

		// Display captured output if conditions are met and it wasn't an fo startup failure.
		if showCaptured && !isActualFoStartupFailure {
			summary := task.RenderSummary() // Get summary of errors/warnings from command output.
			if summary != "" {
				fmt.Print(summary)
			}

			hasActualRenderableOutput := false
			task.OutputLinesLock() // Lock before iterating over task.OutputLines.
			for _, l := range task.OutputLines {
				// Check if the line is actual command output, not an internal fo error message.
				// This uses the De Morgan's law corrected logic.
				if l.Type != design.TypeError || // It's not an error, OR
					// It IS a TypeError, but NOT one of fo's internal startup/pipe error messages.
					(!strings.HasPrefix(l.Content, "Error starting command") &&
						!strings.HasPrefix(l.Content, "Error creating stdout pipe") &&
						!strings.HasPrefix(l.Content, "Error creating stderr pipe") &&
						!strings.HasPrefix(l.Content, "[fo] ")) {
					hasActualRenderableOutput = true
					break
				}
			}
			task.OutputLinesUnlock() // Unlock after iteration.

			if hasActualRenderableOutput {
				fmt.Println(designCfg.GetColor("Muted"), "--- Captured output: ---", designCfg.ResetColor())
				task.OutputLinesLock() // Lock again for rendering.
				for _, line := range task.OutputLines {
					// RenderOutputLine will handle plain rendering for fo's own startup errors if they made it here.
					fmt.Println(task.RenderOutputLine(line))
				}
				task.OutputLinesUnlock() // Unlock.
			} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && summary == "" {
				// If no specific command output but status indicates issues, re-render summary.
				summary = task.RenderSummary()
				if summary != "" {
					fmt.Print(summary)
				}
			}
		} else if !showCaptured && (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
			// If not showing captured output, but there were errors/warnings from the command (not fo startup), show summary.
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	} else { // Stream mode
		// In stream mode, output was live. Only show summary for issues (excluding fo startup issues).
		if (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	}

	fmt.Println(task.RenderEndLine()) // Print the task end line.
	return exitCode
}

// executeStreamMode handles command execution when output is streamed live.
// It pipes command's stdout directly and captures/prints its stderr.
func executeStreamMode(cmd *exec.Cmd, task *design.Task, appSettings LocalAppConfig) (int, error) {
	// Attempt to get a pipe for stderr to process it.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		// Fallback if stderr pipe creation fails: send both stdout/stderr directly.
		if appSettings.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode] Error creating stderr pipe, fallback to direct os.Stderr:", err)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr // Both go to fo's stdout/stderr.
		runErr := cmd.Run()    // cmd.Run() calls Start then Wait.
		// Add a generic error to task if pipe creation failed.
		task.AddOutputLine(fmt.Sprintf("[fo] Error setting up stderr pipe for stream mode: %v", err), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		return getExitCode(runErr), runErr // Return error from cmd.Run().
	}
	cmd.Stdout = os.Stdout // Command's stdout goes directly to fo's stdout.

	var wg sync.WaitGroup
	wg.Add(1)
	// Goroutine to read from the command's stderr pipe.
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		// Use a buffer for the scanner, respecting MaxLineLength.
		buffer := make([]byte, 0, bufio.MaxScanTokenSize) // Default initial capacity.
		scanner.Buffer(buffer, appSettings.MaxLineLength) // Set max capacity.

		for scanner.Scan() {
			line := scanner.Text()
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanned line: %s\n", line)
			}
			fmt.Fprintln(os.Stderr, line) // Print stderr line to fo's actual stderr.
			task.AddOutputLine(line, design.TypeDetail, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 2})
		}
		// Handle scanner errors (e.g., line too long).
		if scanErr := scanner.Err(); scanErr != nil {
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner error: %v\n", scanErr)
			}
			if !errors.Is(scanErr, io.EOF) && !strings.Contains(scanErr.Error(), "file already closed") && !strings.Contains(scanErr.Error(), "broken pipe") {
				task.AddOutputLine(fmt.Sprintf("[fo] Error reading stderr in stream mode: %v", scanErr), design.TypeError, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 3})
			}
		} else if appSettings.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner finished without error.")
		}
	}()

	runErr := cmd.Run() // Start and wait for the command.
	wg.Wait()           // Wait for the stderr processing goroutine to finish.
	return getExitCode(runErr), runErr
}

// executeCaptureMode handles command execution when output is captured and processed.
// It uses pipes for stdout/stderr and processes lines with a PatternMatcher.
func executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, appSettings LocalAppConfig) (int, error) {
	var wg sync.WaitGroup
	var bufferExceeded sync.Once // To ensure buffer limit message is logged only once per stream.

	// Create pipe for command's stdout.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stdout pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg) // Print fo's own error to its stderr.
		return 1, err                   // Return error related to pipe creation.
	}

	// Create pipe for command's stderr.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stderr pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg) // Print fo's own error to its stderr.
		return 1, err                   // Return error related to pipe creation.
	}

	// Goroutine function to process output from a pipe (stdout or stderr).
	// It captures appSettings for debug logging.
	processOutputPipe := func(pipe io.ReadCloser, source string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		// Use a buffer for the scanner, respecting MaxLineLength.
		buffer := make([]byte, 0, bufio.MaxScanTokenSize) // Default initial capacity.
		scanner.Buffer(buffer, appSettings.MaxLineLength) // Set max capacity.

		var currentTotalBytes int64
		for scanner.Scan() {
			line := scanner.Text()
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG processOutputPipe %s] Scanned line: %s\n", source, line)
			}

			lineLength := int64(len(line)) // Get length before potentially truncating or adding.

			// Check if total buffer size for this stream would be exceeded.
			if currentTotalBytes+lineLength > appSettings.MaxBufferSize && appSettings.MaxBufferSize > 0 {
				bufferExceeded.Do(func() { // Log only once.
					msg := fmt.Sprintf("[fo] BUFFER LIMIT: %s stream exceeded %dMB. Further output truncated.", source, appSettings.MaxBufferSize/(1024*1024))
					task.AddOutputLine(msg, design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
				})
				break // Stop processing this pipe.
			}
			currentTotalBytes += lineLength

			// Classify the line using the pattern matcher.
			lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
			// Reclassify unclassified stderr lines as "info" rather than "detail".
			if source == "stderr" && lineType == design.TypeDetail {
				lineType = design.TypeInfo
				lineContext.Importance = 3 // Give stderr info slightly higher importance.
			}
			task.AddOutputLine(line, lineType, lineContext) // This is now mutex-protected.
			task.UpdateTaskContext()                        // This is also mutex-protected.
		}
		// Handle scanner errors (e.g., line too long, read errors).
		if scanErr := scanner.Err(); scanErr != nil {
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG processOutputPipe %s] Scanner error: %v\n", source, scanErr)
			}
			if errors.Is(scanErr, bufio.ErrTooLong) {
				// Log line limit exceeded message (also once via bufferExceeded).
				bufferExceeded.Do(func() {
					msg := fmt.Sprintf("[fo] LINE LIMIT: Max line length (%d KB) exceeded in %s. Line truncated.", appSettings.MaxLineLength/1024, source)
					task.AddOutputLine(msg, design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
				})
			} else if !errors.Is(scanErr, io.EOF) && !strings.Contains(scanErr.Error(), "file already closed") && !strings.Contains(scanErr.Error(), "broken pipe") {
				// Log other significant scanner errors.
				errMsg := fmt.Sprintf("[fo] Error reading %s: %v", source, scanErr)
				task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
			}
		} else if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG processOutputPipe %s] Scanner finished without error.\n", source)
		}
	}

	wg.Add(2) // Two goroutines for stdout and stderr.
	go processOutputPipe(stdoutPipe, "stdout")
	go processOutputPipe(stderrPipe, "stderr")

	// Start the command. This is where "command not found" errors typically occur.
	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg) // Print fo's own startup error to its actual stderr.
		wg.Wait()                       // Wait for pipe processors to finish.
		return getExitCode(err), err    // Return the error from cmd.Start().
	}

	cmdWaitErr := cmd.Wait() // Wait for the command to complete.
	wg.Wait()                // Wait for all output processing goroutines to finish.
	return getExitCode(cmdWaitErr), cmdWaitErr
}

// getExitCode extracts a numerical exit code from an error.
// It handles *exec.ExitError and common "command not found" scenarios.
func getExitCode(err error) int {
	if err == nil {
		return 0 // Success.
	}
	// If it's an *exec.ExitError, the command ran and exited with a status.
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	// Check for "command not found" type errors.
	// exec.ErrNotFound is the most portable check.
	if errors.Is(err, exec.ErrNotFound) ||
		strings.Contains(err.Error(), "executable file not found") ||
		(runtime.GOOS != "windows" && strings.Contains(err.Error(), "no such file or directory")) { // "no such file" can be ambiguous but often means not found on Unix.
		return 127 // Standard exit code for command not found.
	}
	return 1 // Generic error code for other types of errors (e.g., I/O errors).
}

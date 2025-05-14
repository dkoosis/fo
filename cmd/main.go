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

type LocalAppConfig struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool // Effective NoTimer after all flags/configs
	NoColor       bool // Effective NoColor (IsMonochrome)
	CI            bool // Effective CI mode
	Debug         bool
	MaxBufferSize int64
	MaxLineLength int
}

var versionFlag bool
var cliFlagsGlobal config.CliFlags

func main() {
	parseFlagsIntoGlobal()

	if versionFlag {
		fmt.Printf("fo version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	fileAppConfig := config.LoadConfig() // Base from YAML
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1)
	}
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(fileAppConfig, cmdArgs[0]) // Presets modify fileAppConfig
	}

	// Convert fileAppConfig (with presets) to LocalAppConfig for behavioral settings
	behavioralSettings := convertAppConfigToLocal(fileAppConfig)

	// Override behavioral settings with CLI flags
	if cliFlagsGlobal.Label != "" {
		behavioralSettings.Label = cliFlagsGlobal.Label
	}
	if cliFlagsGlobal.StreamSet {
		behavioralSettings.Stream = cliFlagsGlobal.Stream
	}
	if cliFlagsGlobal.ShowOutputSet && cliFlagsGlobal.ShowOutput != "" {
		behavioralSettings.ShowOutput = cliFlagsGlobal.ShowOutput
	}
	if cliFlagsGlobal.DebugSet { // CLI debug flag overrides all
		behavioralSettings.Debug = cliFlagsGlobal.Debug
		fileAppConfig.Debug = cliFlagsGlobal.Debug // Ensure this is passed to MergeWithFlags
	}
	if cliFlagsGlobal.MaxBufferSize > 0 {
		behavioralSettings.MaxBufferSize = cliFlagsGlobal.MaxBufferSize
	}
	if cliFlagsGlobal.MaxLineLength > 0 {
		behavioralSettings.MaxLineLength = cliFlagsGlobal.MaxLineLength
	}

	// Get final design configuration (styling)
	// MergeWithFlags considers fileAppConfig (with presets & updated Debug) and CLI flags.
	finalDesignConfig := config.MergeWithFlags(fileAppConfig, cliFlagsGlobal)

	// Update behavioralSettings with final decisions on NoTimer, NoColor, CI from finalDesignConfig
	// This ensures consistency if executeCommand logic depends on these.
	behavioralSettings.NoTimer = finalDesignConfig.Style.NoTimer
	behavioralSettings.NoColor = finalDesignConfig.IsMonochrome
	behavioralSettings.CI = finalDesignConfig.IsMonochrome && finalDesignConfig.Style.NoTimer // Common def for CI

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	exitCode := executeCommand(ctx, cancel, sigChan, behavioralSettings, finalDesignConfig, cmdArgs)
	if behavioralSettings.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d).\nBehavioral Config: %+v\n", exitCode, behavioralSettings)
	}
	os.Exit(exitCode)
}

func convertAppConfigToLocal(appCfg *config.AppConfig) LocalAppConfig {
	return LocalAppConfig{
		Label: appCfg.Label, Stream: appCfg.Stream, ShowOutput: appCfg.ShowOutput,
		NoTimer: appCfg.NoTimer, NoColor: appCfg.NoColor, CI: appCfg.CI, Debug: appCfg.Debug,
		MaxBufferSize: appCfg.MaxBufferSize, MaxLineLength: appCfg.MaxLineLength,
	}
}

func parseFlagsIntoGlobal() {
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
		valid := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !valid[cliFlagsGlobal.ShowOutput] {
			fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\nValid values are: on-fail, always, never\n", cliFlagsGlobal.ShowOutput)
			flag.Usage()
			os.Exit(1)
		}
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

func executeCommand(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal,
	appSettings LocalAppConfig, designCfg *design.Config, cmdArgs []string) int {
	labelToUse := appSettings.Label
	if labelToUse == "" {
		labelToUse = filepath.Base(cmdArgs[0])
	}

	patternMatcher := design.NewPatternMatcher(designCfg)
	intent := patternMatcher.DetectCommandIntent(cmdArgs[0], cmdArgs[1:])
	task := design.NewTask(labelToUse, intent, cmdArgs[0], cmdArgs[1:], designCfg)

	fmt.Println(task.RenderStartLine()) // RenderStartLine uses task.Config (which is designCfg)

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // For signal propagation to process group

	cmdDone := make(chan struct{})
	go func() { // Signal handling goroutine
		defer close(cmdDone)
		select {
		case sig := <-sigChan:
			if cmd.Process == nil { // Command hasn't started or already finished
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Process is nil, canceling context.")
				}
				cancel()
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
			} else { // Fallback if Getpgid fails
				if appSettings.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Failed to get PGID for PID %d (%v), sending to PID directly.\n", cmd.Process.Pid, err)
				}
				_ = cmd.Process.Signal(sig)
			}
			select { // Wait for command to react or timeout
			case <-cmdDone:
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received after signal forwarding.")
				}
			case <-time.After(2 * time.Second): // Grace period
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Timeout after signal, ensuring process is killed.")
				}
				if cmd.Process != nil && cmd.ProcessState == nil { // If still running
					pgidKill, errKill := syscall.Getpgid(cmd.Process.Pid)
					if errKill == nil {
						_ = syscall.Kill(-pgidKill, syscall.SIGKILL)
					} else {
						_ = cmd.Process.Kill()
					}
				}
				cancel() // Cancel context as a final measure
			}
			return
		case <-ctx.Done(): // Context was cancelled elsewhere (e.g. timeout if implemented)
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
	var cmdExecError error // Captures errors from cmd.Start(), cmd.Run(), cmd.Wait()

	if appSettings.Stream {
		exitCode, cmdExecError = executeStreamMode(cmd, task, appSettings)
	} else {
		exitCode, cmdExecError = executeCaptureMode(cmd, task, patternMatcher, appSettings)
	}

	task.Complete(exitCode) // task.Config is designCfg

	// Determine if the error was fo failing to start/pipe, not the command itself misbehaving.
	isFoStartupError := cmdExecError != nil && exitCode != 0 // If there was an exec error and command failed

	if !appSettings.Stream {
		showCaptured := false
		switch appSettings.ShowOutput {
		case "always":
			showCaptured = true
		case "on-fail":
			if exitCode != 0 {
				showCaptured = true
			}
			// "never" remains false
		}

		// Only show "Captured output" if it's NOT an fo startup error (like command not found)
		// AND the showCaptured flag allows it.
		if showCaptured && !isFoStartupError {
			summary := task.RenderSummary() // RenderSummary is updated to exclude fo internal errors
			if summary != "" {
				fmt.Print(summary)
			}

			// Check if there's actual command output to display, not just fo's internal error messages
			// that might have been added to task.OutputLines by executeCaptureMode.
			// RenderOutputLine is updated to render fo's internal errors plainly.
			hasActualRenderableOutput := false
			for _, l := range task.OutputLines {
				if !(l.Type == design.TypeError &&
					(strings.HasPrefix(l.Content, "Error starting command") ||
						strings.HasPrefix(l.Content, "Error creating stdout pipe") ||
						strings.HasPrefix(l.Content, "Error creating stderr pipe") ||
						strings.HasPrefix(l.Content, "[fo] "))) {
					hasActualRenderableOutput = true
					break
				}
			}

			if hasActualRenderableOutput {
				fmt.Println(designCfg.GetColor("Muted"), "--- Captured output: ---", designCfg.ResetColor())
				for _, line := range task.OutputLines {
					// RenderOutputLine will handle plain rendering for fo's own startup errors if they are present
					fmt.Println(task.RenderOutputLine(line))
				}
			} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && summary == "" {
				// If no specific command output but still an error/warning from the command (not fo startup),
				// ensure summary (if any) is shown.
				summary = task.RenderSummary()
				if summary != "" {
					fmt.Print(summary)
				}
			}
		} else if !showCaptured && (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isFoStartupError {
			// If not showing captured output, but there were non-startup errors/warnings from the command
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	} else { // Stream mode
		// In stream mode, output was live. Only show summary for issues (excluding fo startup issues).
		if (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isFoStartupError {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	}

	fmt.Println(task.RenderEndLine()) // RenderEndLine uses task.Config (designCfg)
	return exitCode
}

// executeStreamMode returns (exitCode, error from cmd.Run())
func executeStreamMode(cmd *exec.Cmd, task *design.Task, appSettings LocalAppConfig) (int, error) {
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if appSettings.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode] Error creating stderr pipe, fallback to direct os.Stderr:", err)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr := cmd.Run() // This tries to Start then Wait. If Start fails, runErr reflects that.
		// Add a generic error to task if pipe creation failed,
		// as we can't capture specific details if we fall back.
		task.AddOutputLine(fmt.Sprintf("Error setting up stderr pipe for stream mode: %v", err), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		return getExitCode(runErr), runErr // The error is from cmd.Run()
	}
	cmd.Stdout = os.Stdout // Stdout goes directly to os.Stdout

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		// Use appSettings for MaxLineLength, similar to capture mode
		scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), appSettings.MaxLineLength)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line) // Print to actual stderr for live view
			// Add to task.OutputLines for potential summary or if logic changes
			task.AddOutputLine(line, design.TypeInfo, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 2})
		}
		if scanErr := scanner.Err(); scanErr != nil {
			// Avoid logging benign EOF or pipe closure errors if command completes successfully.
			if !errors.Is(scanErr, io.EOF) && !strings.Contains(scanErr.Error(), "file already closed") && !strings.Contains(scanErr.Error(), "broken pipe") {
				task.AddOutputLine(fmt.Sprintf("Error reading stderr in stream mode: %v", scanErr), design.TypeError, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 3})
			}
		}
	}()

	runErr := cmd.Run()                // cmd.Run() will Start then Wait.
	wg.Wait()                          // Wait for stderr processing to complete.
	return getExitCode(runErr), runErr // The error is from cmd.Run()
}

// executeCaptureMode returns (exitCode, error from cmd.Start() or cmd.Wait())
func executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, appSettings LocalAppConfig) (int, error) {
	var wg sync.WaitGroup
	var bufferExceeded sync.Once

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating stdout pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg) // Print fo's own error to its stderr
		return 1, err                   // Return error related to pipe creation
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating stderr pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg) // Print fo's own error to its stderr
		return 1, err                   // Return error related to pipe creation
	}

	processOutputPipe := func(pipe io.ReadCloser, source string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), appSettings.MaxLineLength)
		var currentTotalBytes int64
		for scanner.Scan() {
			line := scanner.Text()
			lineLength := int64(len(line))
			if currentTotalBytes+lineLength > appSettings.MaxBufferSize {
				bufferExceeded.Do(func() {
					msg := fmt.Sprintf("[fo] BUFFER LIMIT: %s stream exceeded %dMB. Further output truncated.", source, appSettings.MaxBufferSize/(1024*1024))
					task.AddOutputLine(msg, design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
				})
				break // Stop processing this pipe
			}
			currentTotalBytes += lineLength
			lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
			if source == "stderr" && lineType == design.TypeDetail { // Reclassify unclassified stderr
				lineType = design.TypeInfo
				lineContext.Importance = 3
			}
			task.AddOutputLine(line, lineType, lineContext)
			task.UpdateTaskContext()
			// No direct printing here in capture mode; executeCommand handles based on ShowOutput
		}
		if errScan := scanner.Err(); errScan != nil {
			if errors.Is(errScan, bufio.ErrTooLong) {
				bufferExceeded.Do(func() { // Ensure buffer exceeded message is logged if ErrTooLong occurs
					msg := fmt.Sprintf("[fo] LINE LIMIT: Max line length (%d KB) exceeded in %s. Line truncated.", appSettings.MaxLineLength/1024, source)
					task.AddOutputLine(msg, design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
				})
			} else if !errors.Is(errScan, io.EOF) && !strings.Contains(errScan.Error(), "file already closed") && !strings.Contains(errScan.Error(), "broken pipe") {
				// Log other scanner errors if they are not typical pipe closure issues
				errMsg := fmt.Sprintf("Error reading %s: %v", source, errScan)
				task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
			}
		}
	}

	wg.Add(2)
	go processOutputPipe(stdoutPipe, "stdout")
	go processOutputPipe(stderrPipe, "stderr")

	if err := cmd.Start(); err != nil { // This is the primary "command not found" or startup failure point
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		// Add to task.OutputLines so it's recorded, but it's an fo error.
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg) // Print fo's own startup error to its actual stderr
		return getExitCode(err), err    // Return the error from cmd.Start()
	}

	cmdWaitErr := cmd.Wait()                   // Wait for the command to finish
	wg.Wait()                                  // Wait for all output processing goroutines to finish
	return getExitCode(cmdWaitErr), cmdWaitErr // Return error from cmd.Wait() (if any)
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	// Check for common "command not found" scenarios.
	// exec.ErrNotFound is a portable way to check this.
	if errors.Is(err, exec.ErrNotFound) ||
		strings.Contains(err.Error(), "executable file not found") ||
		(runtime.GOOS != "windows" && strings.Contains(err.Error(), "no such file or directory")) { // "no such file" can be ambiguous
		return 127 // Standard for command not found
	}
	return 1 // Generic error for other cases (e.g., I/O errors if not ExitError)
}

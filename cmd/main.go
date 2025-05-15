package main

import (
	"bufio"
	"bytes" // Import bytes for buffer
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
	behavioralSettings.NoTimer = finalDesignConfig.Style.NoTimer
	behavioralSettings.NoColor = finalDesignConfig.IsMonochrome
	behavioralSettings.CI = finalDesignConfig.IsMonochrome && finalDesignConfig.Style.NoTimer
	if fileAppConfig.Debug {
		behavioralSettings.Debug = true
	}
	if cliFlagsGlobal.DebugSet {
		behavioralSettings.Debug = cliFlagsGlobal.Debug
	}

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
		validValues := map[string]bool{"on-fail": true, "always": true, "never": true}
		if !validValues[cliFlagsGlobal.ShowOutput] {
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

	fmt.Println(task.RenderStartLine())

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
				_ = syscall.Kill(-pgid, sig.(syscall.Signal))
			} else {
				if appSettings.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Failed to get PGID for PID %d (%v), sending to PID directly.\n", cmd.Process.Pid, err)
				}
				_ = cmd.Process.Signal(sig)
			}
			select {
			case <-cmdDone:
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received after signal forwarding.")
				}
			case <-time.After(2 * time.Second):
				if appSettings.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Timeout after signal, ensuring process is killed.")
				}
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
		case <-cmdDone:
			if appSettings.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received, command finished naturally.")
			}
			return
		}
	}()

	var exitCode int
	var cmdRunError error
	var isActualFoStartupFailure bool

	if appSettings.Stream {
		exitCode, cmdRunError = executeStreamMode(cmd, task, appSettings)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	} else {
		exitCode, cmdRunError = executeCaptureMode(cmd, task, patternMatcher, appSettings)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	}

	task.Complete(exitCode)

	if !appSettings.Stream {
		showCaptured := false
		switch appSettings.ShowOutput {
		case "always":
			showCaptured = true
		case "on-fail":
			if exitCode != 0 {
				showCaptured = true
			}
		}

		if showCaptured && !isActualFoStartupFailure {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}

			hasActualRenderableOutput := false
			task.OutputLinesLock()
			for _, l := range task.OutputLines {
				if l.Type != design.TypeError ||
					(!strings.HasPrefix(l.Content, "Error starting command") &&
						!strings.HasPrefix(l.Content, "Error creating stdout pipe") &&
						!strings.HasPrefix(l.Content, "Error creating stderr pipe") &&
						!strings.HasPrefix(l.Content, "[fo] ")) {
					hasActualRenderableOutput = true
					break
				}
			}
			task.OutputLinesUnlock()

			if hasActualRenderableOutput {
				fmt.Println(designCfg.GetColor("Muted"), "--- Captured output: ---", designCfg.ResetColor())
				task.OutputLinesLock()
				for _, line := range task.OutputLines {
					fmt.Println(task.RenderOutputLine(line))
				}
				task.OutputLinesUnlock()
			} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && summary == "" {
				summary = task.RenderSummary()
				if summary != "" {
					fmt.Print(summary)
				}
			}
		} else if !showCaptured && (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	} else {
		if (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
		}
	}

	fmt.Println(task.RenderEndLine())
	return exitCode
}

func executeStreamMode(cmd *exec.Cmd, task *design.Task, appSettings LocalAppConfig) (int, error) {
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if appSettings.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode] Error creating stderr pipe, fallback to direct os.Stderr:", err)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr := cmd.Run()
		task.AddOutputLine(fmt.Sprintf("[fo] Error setting up stderr pipe for stream mode: %v", err), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		return getExitCode(runErr), runErr
	}
	cmd.Stdout = os.Stdout

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		buffer := make([]byte, 0, bufio.MaxScanTokenSize)
		scanner.Buffer(buffer, appSettings.MaxLineLength)

		for scanner.Scan() {
			line := scanner.Text()
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanned line: %s\n", line)
			}
			fmt.Fprintln(os.Stderr, line)
			task.AddOutputLine(line, design.TypeDetail, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 2})
		}
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

	runErr := cmd.Run() // This calls Start then Wait.
	wg.Wait()
	return getExitCode(runErr), runErr
}

// executeCaptureMode uses io.Copy to bytes.Buffer to capture full output before processing.
func executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, appSettings LocalAppConfig) (int, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stdout pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg)
		return 1, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stderr pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg)
		_ = stdoutPipe.Close() // Close the already opened stdout pipe
		return 1, err
	}

	var stdoutBuffer, stderrBuffer bytes.Buffer
	var wgRead sync.WaitGroup
	wgRead.Add(2) // For two goroutines copying stdout and stderr

	var errStdoutCopy, errStderrCopy error

	go func() {
		defer wgRead.Done()
		if appSettings.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Copying stdoutPipe")
		}
		_, errStdoutCopy = io.Copy(&stdoutBuffer, stdoutPipe)
		if errStdoutCopy != nil && !errors.Is(errStdoutCopy, io.EOF) { // EOF is expected
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Error copying stdout: %v\n", errStdoutCopy)
			}
		} else if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Finished copying stdoutPipe (len: %d)\n", stdoutBuffer.Len())
		}
	}()

	go func() {
		defer wgRead.Done()
		if appSettings.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Copying stderrPipe")
		}
		_, errStderrCopy = io.Copy(&stderrBuffer, stderrPipe)
		if errStderrCopy != nil && !errors.Is(errStderrCopy, io.EOF) { // EOF is expected
			if appSettings.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Error copying stderr: %v\n", errStderrCopy)
			}
		} else if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Finished copying stderrPipe (len: %d)\n", stderrBuffer.Len())
		}
	}()

	// Start the command
	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(os.Stderr, errMsg)
		// Pipes are associated with cmd; cmd.Wait() will handle their closure if Start was successful.
		// If Start fails, the pipes might not be fully "live" from the child's perspective.
		// Explicitly closing them here might be redundant or cause issues if cmd.Wait() is not called.
		// Let cmd.Wait() (or its absence on Start error) manage pipe state.
		// However, we must ensure our reading goroutines (wgRead) don't hang.
		// Closing our ends of the pipes can help unblock io.Copy.
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		wgRead.Wait() // Ensure goroutines finish if Start fails.
		return getExitCode(err), err
	}

	cmdWaitErr := cmd.Wait() // Wait for command to exit; this closes its side of pipes.
	wgRead.Wait()            // Wait for io.Copy goroutines to complete (they will see EOF).

	if appSettings.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] stdout captured (len %d): %s\n", stdoutBuffer.Len(), stdoutBuffer.String())
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] stderr captured (len %d): %s\n", stderrBuffer.Len(), stderrBuffer.String())
	}

	var bufferLimitLogged sync.Once

	// Process stdoutData
	scanner := bufio.NewScanner(&stdoutBuffer) // Use &stdoutBuffer (bytes.Buffer implements io.Reader)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), appSettings.MaxLineLength)
	var currentTotalStdoutBytes int64
	for scanner.Scan() {
		line := scanner.Text()
		if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode STDOUT_SCAN] Scanned line: %s\n", line)
		}
		lineLength := int64(len(line))
		if appSettings.MaxBufferSize > 0 && currentTotalStdoutBytes+lineLength > appSettings.MaxBufferSize {
			bufferLimitLogged.Do(func() {
				task.AddOutputLine(fmt.Sprintf("[fo] BUFFER LIMIT: stdout stream exceeded %dMB. Further output truncated.", appSettings.MaxBufferSize/(1024*1024)), design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
			})
			break
		}
		currentTotalStdoutBytes += lineLength
		lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
		task.AddOutputLine(line, lineType, lineContext)
	}
	if errScan := scanner.Err(); errScan != nil {
		if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode STDOUT_SCAN] Scanner error: %v\n", errScan)
		}
		if errors.Is(errScan, bufio.ErrTooLong) {
			bufferLimitLogged.Do(func() {
				task.AddOutputLine(fmt.Sprintf("[fo] LINE LIMIT: Max line length (%d KB) exceeded in stdout. Line truncated.", appSettings.MaxLineLength/1024), design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
			})
		} else if !errors.Is(errScan, io.EOF) { // Don't log EOF as an error from scanner
			task.AddOutputLine(fmt.Sprintf("[fo] Error scanning stdout buffer: %v", errScan), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
		}
	}

	// Process stderrData
	scanner = bufio.NewScanner(&stderrBuffer) // Use &stderrBuffer
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), appSettings.MaxLineLength)
	var currentTotalStderrBytes int64
	for scanner.Scan() {
		line := scanner.Text()
		if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode STDERR_SCAN] Scanned line: %s\n", line)
		}
		lineLength := int64(len(line))
		if appSettings.MaxBufferSize > 0 && currentTotalStderrBytes+lineLength > appSettings.MaxBufferSize {
			bufferLimitLogged.Do(func() {
				task.AddOutputLine(fmt.Sprintf("[fo] BUFFER LIMIT: stderr stream exceeded %dMB. Further output truncated.", appSettings.MaxBufferSize/(1024*1024)), design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
			})
			break
		}
		currentTotalStderrBytes += lineLength
		lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
		if lineType == design.TypeDetail {
			lineType = design.TypeInfo
			lineContext.Importance = 3
		}
		task.AddOutputLine(line, lineType, lineContext)
	}
	if errScan := scanner.Err(); errScan != nil {
		if appSettings.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode STDERR_SCAN] Scanner error: %v\n", errScan)
		}
		if errors.Is(errScan, bufio.ErrTooLong) {
			bufferLimitLogged.Do(func() {
				task.AddOutputLine(fmt.Sprintf("[fo] LINE LIMIT: Max line length (%d KB) exceeded in stderr. Line truncated.", appSettings.MaxLineLength/1024), design.TypeWarning, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
			})
		} else if !errors.Is(errScan, io.EOF) { // Don't log EOF as an error from scanner
			task.AddOutputLine(fmt.Sprintf("[fo] Error scanning stderr buffer: %v", errScan), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
		}
	}

	task.UpdateTaskContext()

	if cmdWaitErr != nil {
		return getExitCode(cmdWaitErr), cmdWaitErr
	}
	// Check errors from io.Copy if cmdWaitErr was nil
	if errStdoutCopy != nil && !errors.Is(errStdoutCopy, io.EOF) {
		task.AddOutputLine(fmt.Sprintf("[fo] Final stdout copy error: %v", errStdoutCopy), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
		return 1, errStdoutCopy
	}
	if errStderrCopy != nil && !errors.Is(errStderrCopy, io.EOF) {
		task.AddOutputLine(fmt.Sprintf("[fo] Final stderr copy error: %v", errStderrCopy), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
		return 1, errStderrCopy
	}

	return 0, nil // Success
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	if errors.Is(err, exec.ErrNotFound) ||
		strings.Contains(err.Error(), "executable file not found") ||
		(runtime.GOOS != "windows" && strings.Contains(err.Error(), "no such file or directory")) {
		return 127
	}
	return 1
}

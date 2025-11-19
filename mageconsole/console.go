package mageconsole

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/davidkoosis/fo/internal/design"
)

type ConsoleConfig struct {
	ThemeName      string
	UseBoxes       bool
	UseBoxesSet    bool
	InlineProgress bool
	InlineSet      bool
	Monochrome     bool
	ShowTimer      bool
	ShowTimerSet   bool
	ShowOutputMode string
	Stream         bool
	Debug          bool
	MaxBufferSize  int64
	MaxLineLength  int
	Design         *design.Config
	Out            io.Writer // Output writer, defaults to os.Stdout
	Err            io.Writer // Error writer, defaults to os.Stderr
}

// Line represents a classified line of command output.
// This is the public-facing type that doesn't leak internal design package types.
type Line struct {
	Content   string
	Type      string // "detail", "error", "warning", "success", "info", "progress"
	Timestamp time.Time
}

type TaskResult struct {
	Label    string
	Intent   string
	Status   string
	Duration time.Duration
	ExitCode int
	Lines    []Line
	Err      error
}

type Console struct {
	cfg        ConsoleConfig
	designConf *design.Config
}

func DefaultConsole() *Console {
	return NewConsole(ConsoleConfig{})
}

func NewConsole(cfg ConsoleConfig) *Console {
	normalized := normalizeConfig(cfg)
	return &Console{cfg: normalized, designConf: resolveDesignConfig(normalized)}
}

// Run executes a command and returns the result.
//
// Error semantics:
//   - Returns (result, nil) when the command runs successfully (exit code 0)
//   - Returns (result, error) when the command runs but exits non-zero;
//     the error wraps the underlying exec.ExitError
//   - Returns (result, error) for infrastructure failures (command not found,
//     IO errors, context cancelled)
//
// Note: TaskResult is always non-nil. Even for infrastructure failures, the
// result contains useful information like duration, label, and any captured
// internal error messages. Use TaskResult.ExitCode (127 for command not found,
// 1 for other failures) and TaskResult.Err for failure details.
//
// Use errors.Is(err, exec.ErrNotFound) to check for missing commands.
func (c *Console) Run(label, command string, args ...string) (*TaskResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, getInterruptSignals()...)
	// Note: signal.Stop is called in the signal handler goroutine (see runContext)

	return c.runContext(ctx, cancel, sigChan, label, command, args)
}

// ErrNonZeroExit is returned when a command completes but exits with a non-zero code.
// Use errors.Is(err, ErrNonZeroExit) to check for this condition.
var ErrNonZeroExit = errors.New("command exited with non-zero code")

// ExitCodeError wraps an exit code for programmatic access.
// Use errors.As(err, &ExitCodeError{}) to extract the exit code from RunSimple errors.
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// RunSimple executes a command and returns only an error.
// This is a convenience wrapper around Run for simple use cases where you
// only need to know success vs failure.
//
// Returns nil on success (exit code 0).
// Returns ErrNonZeroExit (wrapped with ExitCodeError) if the command exits
// with non-zero code.
// Returns other errors for infrastructure failures.
//
// To check for non-zero exit and extract the code:
//
//	if errors.Is(err, ErrNonZeroExit) {
//	    var exitErr mageconsole.ExitCodeError
//	    if errors.As(err, &exitErr) {
//	        fmt.Printf("Exit code: %d\n", exitErr.Code)
//	    }
//	}
//
// For detailed results including captured output, use Run() instead.
func (c *Console) RunSimple(command string, args ...string) error {
	_, err := c.Run("", command, args...)
	if err == nil {
		return nil
	}

	// Map exec.ExitError to our wrapper error with extractable exit code
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := getExitCode(err, c.cfg.Debug)
		return fmt.Errorf("%w: %w", ErrNonZeroExit, ExitCodeError{Code: code})
	}
	return err // Infrastructure error, pass through
}

func (c *Console) runContext(
	ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal,
	label, command string, args []string,
) (*TaskResult, error) {
	labelToUse := label
	if labelToUse == "" {
		labelToUse = filepath.Base(command)
	}

	designCfg := design.DeepCopyConfig(c.designConf)

	patternMatcher := design.NewPatternMatcher(designCfg)
	intent := patternMatcher.DetectCommandIntent(command, args)
	task := design.NewTask(labelToUse, intent, command, args, designCfg)

	useInlineProgress := designCfg.Style.UseInlineProgress && c.cfg.InlineProgress && !c.cfg.Stream

	progress := design.NewInlineProgress(task, c.cfg.Debug, c.cfg.Out)

	// Set up cursor restoration at the outermost level for inline progress
	if useInlineProgress {
		enableSpinner := !designCfg.Style.NoSpinner
		if enableSpinner && design.IsInteractiveTerminal() && !designCfg.IsMonochrome {
			// Hide cursor at start, restore on any exit path
			_, _ = c.cfg.Out.Write([]byte("\033[?25l"))
			defer func() {
				_, _ = c.cfg.Out.Write([]byte("\033[?25h\n"))
			}()
		}
		progress.Start(ctx, enableSpinner)
	} else {
		_, _ = c.cfg.Out.Write([]byte(task.RenderStartLine() + "\n"))
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = os.Environ()
	setProcessGroup(cmd)

	cmdDone := make(chan struct{})

	// Goroutine: Handle signals
	signalHandlerDone := make(chan struct{})
	go func() {
		defer func() {
			signal.Stop(sigChan)
			close(signalHandlerDone)
		}()
		select {
		case sig := <-sigChan:
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Received signal %v\n", sig)
				processStateStr := "nil"
				if cmd.ProcessState != nil {
					processStateStr = fmt.Sprintf("%+v", cmd.ProcessState)
				}
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Process state: %s\n", processStateStr)
			}
			if cmd.Process == nil {
				if c.cfg.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Process is nil, canceling context.")
				}
				cancel()
				return
			}
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Received signal %v for PID %d. Forwarding...\n", sig, cmd.Process.Pid)
			}
			if err := killProcessGroup(cmd, sig); err != nil && c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Error killing process group: %v\n", err)
			}
			select {
			case <-cmdDone:
				if c.cfg.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received after signal forwarding.")
				}
			case <-time.After(2 * time.Second):
				if c.cfg.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Timeout after signal, ensuring process is killed.")
				}
				if cmd.Process != nil && cmd.ProcessState == nil {
					_ = killProcessGroupWithSIGKILL(cmd)
				}
				cancel()
			}
		case <-ctx.Done():
			if c.cfg.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Context done, ensuring process is killed if running.")
			}
			if cmd.Process != nil && cmd.ProcessState == nil {
				_ = killProcessGroupWithSIGKILL(cmd)
			}
		case <-cmdDone:
			if c.cfg.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received, command finished naturally.")
			}
		}
	}()

	var exitCode int
	var cmdRunError error
	var isActualFoStartupFailure bool

	// Execute command (these functions call cmd.Start() and cmd.Wait())
	// They will close cmdDone when cmd.Wait() completes
	if c.cfg.Stream {
		exitCode, cmdRunError = c.executeStreamMode(cmd, task, cmdDone)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	} else {
		exitCode, cmdRunError = c.executeCaptureMode(cmd, task, patternMatcher, cmdDone)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	}

	// Wait for signal handler to finish
	<-signalHandlerDone

	task.Complete(exitCode)

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr,
			"[DEBUG executeCommand] CI=%t, exitCode=%d, task.Status=%s, isActualFoStartupFailure=%t\n",
			c.cfg.Monochrome && !c.cfg.ShowTimer, exitCode, task.Status, isActualFoStartupFailure)
	}

	if useInlineProgress {
		status := design.StatusSuccess
		if exitCode != 0 {
			status = design.StatusError
		} else if task.Status == design.StatusWarning {
			status = design.StatusWarning
		}
		progress.Complete(status)
	}

	if !c.cfg.Stream {
		c.renderCapturedOutput(task, exitCode, isActualFoStartupFailure)
	} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
		summary := task.RenderSummary()
		if summary != "" {
			_, _ = c.cfg.Out.Write([]byte(summary))
		}
	}

	if !useInlineProgress {
		_, _ = c.cfg.Out.Write([]byte(task.RenderEndLine() + "\n"))
	}

	// Convert design.OutputLine to mageconsole.Line
	lines := make([]Line, len(task.OutputLines))
	for i, ol := range task.OutputLines {
		lines[i] = Line{
			Content:   ol.Content,
			Type:      ol.Type,
			Timestamp: ol.Timestamp,
		}
	}

	return &TaskResult{
		Label:    task.Label,
		Intent:   task.Intent,
		Status:   task.Status,
		Duration: task.Duration,
		ExitCode: exitCode,
		Lines:    lines,
		Err:      cmdRunError,
	}, cmdRunError
}

func (c *Console) renderCapturedOutput(task *design.Task, exitCode int, isActualFoStartupFailure bool) {
	showCaptured := false
	switch c.cfg.ShowOutputMode {
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
			_, _ = c.cfg.Out.Write([]byte(summary))
		}

		hasActualRenderableOutput := false
		task.OutputLinesLock()
		for _, l := range task.OutputLines {
			// Check IsInternal flag first, fall back to string prefix for backwards compatibility
			isInternal := l.Context.IsInternal ||
				(l.Type == design.TypeError && (strings.HasPrefix(l.Content, "Error starting command") ||
					strings.HasPrefix(l.Content, "Error creating stdout pipe") ||
					strings.HasPrefix(l.Content, "Error creating stderr pipe") ||
					strings.HasPrefix(l.Content, "[fo] ")))
			if !isInternal {
				hasActualRenderableOutput = true
				break
			}
		}
		task.OutputLinesUnlock()

		if hasActualRenderableOutput {
			_, _ = c.cfg.Out.Write([]byte(task.Config.GetColor("Muted") + "--- Captured output: ---" + task.Config.ResetColor() + "\n"))
			task.OutputLinesLock()
			for _, line := range task.OutputLines {
				_, _ = c.cfg.Out.Write([]byte(task.RenderOutputLine(line) + "\n"))
			}
			task.OutputLinesUnlock()
		} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && summary == "" {
			summary = task.RenderSummary()
			if summary != "" {
				_, _ = c.cfg.Out.Write([]byte(summary))
			}
		}
	} else if !showCaptured && (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
		summary := task.RenderSummary()
		if summary != "" {
			_, _ = c.cfg.Out.Write([]byte(summary))
		}
	}
}

func (c *Console) executeStreamMode(cmd *exec.Cmd, task *design.Task, cmdDone chan struct{}) (int, error) {
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode] Error creating stderr pipe, fallback to direct os.Stderr:", err)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr := cmd.Run()
		close(cmdDone) // Signal that command has finished
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true}
		task.AddOutputLine(
			fmt.Sprintf("[fo] Error setting up stderr pipe for stream mode: %v", err),
			design.TypeError, errCtx)
		exitCode := getExitCode(runErr, c.cfg.Debug)
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return exitCode, runErr
		}
		return exitCode, runErr
	}
	cmd.Stdout = os.Stdout

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		scanner := bufio.NewScanner(stderrPipe)
		buffer := make([]byte, 0, bufio.MaxScanTokenSize)
		scanner.Buffer(buffer, c.cfg.MaxLineLength)

		for scanner.Scan() {
			line := scanner.Text()
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanned line: %s\n", line)
			}
			fmt.Fprintln(c.cfg.Err, line)
			task.AddOutputLine(line, design.TypeDetail, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 2})
		}
		if scanErr := scanner.Err(); scanErr != nil {
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner error: %v\n", scanErr)
			}
			isIgnorable := errors.Is(scanErr, io.EOF) ||
				strings.Contains(scanErr.Error(), "file already closed") ||
				strings.Contains(scanErr.Error(), "broken pipe")
			if !isIgnorable {
				errCtx := design.LineContext{
					CognitiveLoad: design.LoadMedium, Importance: 3, IsInternal: true,
				}
				task.AddOutputLine(
					fmt.Sprintf("[fo] Error reading stderr in stream mode: %v", scanErr),
					design.TypeError, errCtx)
			}
		} else if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner finished without error.")
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), startErr)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)

		_ = stderrPipe.Close()
		waitGroup.Wait()
		close(cmdDone) // Signal that command has finished (failed to start)

		return getExitCode(startErr, c.cfg.Debug), startErr
	}

	runErr := cmd.Wait()
	waitGroup.Wait()
	close(cmdDone) // Signal that command has finished

	exitCode := getExitCode(runErr, c.cfg.Debug)
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return exitCode, runErr
		}
	}

	return exitCode, runErr
}

func (c *Console) executeCaptureMode(
	cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, cmdDone chan struct{},
) (int, error) {
	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Starting in CAPTURE mode\n")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stdout pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)
		return 1, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stderr pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)
		_ = stdoutPipe.Close()
		return 1, err
	}

	var wgRead sync.WaitGroup
	wgRead.Add(2)

	var errStdoutCopy, errStderrCopy error
	var totalBytesRead int64
	maxTotalBytes := c.cfg.MaxBufferSize * 2 // Shared budget: 2x MaxBufferSize across both streams
	var bytesMutex sync.Mutex

	// Helper function to process a pipe line-by-line with classification
	processPipe := func(pipe io.ReadCloser, streamName string, errVar *error) {
		defer wgRead.Done()
		if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Reading %s line-by-line\n", streamName)
		}
		scanner := bufio.NewScanner(pipe)
		buf := make([]byte, 0, bufio.MaxScanTokenSize)
		scanner.Buffer(buf, c.cfg.MaxLineLength)

		truncated := false
		for scanner.Scan() {
			line := scanner.Text()
			lineBytes := int64(len(line))

			// Enforce MaxBufferSize limit (thread-safe check and update)
			// Important: We must continue draining the pipe even after limit is reached
			// to prevent deadlock (child process blocking on write to full pipe buffer)
			bytesMutex.Lock()
			overLimit := totalBytesRead+lineBytes > maxTotalBytes
			if !overLimit {
				totalBytesRead += lineBytes
			}
			bytesMutex.Unlock()

			if overLimit {
				if !truncated && c.cfg.Debug {
					fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] MaxBufferSize limit reached on %s, discarding remaining output\n", streamName)
				}
				truncated = true
				continue // Keep scanning to drain pipe, but don't store
			}

			// Classify and add line immediately (streaming classification)
			lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Line classified as %s: %s\n", lineType, line)
			}
			task.AddOutputLine(line, lineType, lineContext)
		}

		if scanErr := scanner.Err(); scanErr != nil {
			isIgnorable := errors.Is(scanErr, io.EOF) ||
				strings.Contains(scanErr.Error(), "file already closed") ||
				strings.Contains(scanErr.Error(), "broken pipe")
			if !isIgnorable {
				*errVar = scanErr
				if c.cfg.Debug {
					fmt.Fprintf(os.Stderr,
						"[DEBUG executeCaptureMode] Error scanning %s: %v\n", streamName, scanErr)
				}
			}
		} else if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Finished reading %s\n", streamName)
		}
	}

	// Stream line-by-line classification for real-time processing
	go processPipe(stdoutPipe, "stdoutPipe", &errStdoutCopy)
	go processPipe(stderrPipe, "stderrPipe", &errStderrCopy)

	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		wgRead.Wait()
		close(cmdDone) // Signal that command has finished (failed to start)
		return getExitCode(err, c.cfg.Debug), err
	}

	runErr := cmd.Wait()
	wgRead.Wait()
	close(cmdDone) // Signal that command has finished

	// Note: Output was already classified line-by-line during capture above
	// Report any scanning errors
	isIgnorableErr := func(err error) bool {
		return errors.Is(err, io.EOF) ||
			strings.Contains(err.Error(), "file already closed") ||
			strings.Contains(err.Error(), "broken pipe")
	}
	errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true}
	if errStdoutCopy != nil && !isIgnorableErr(errStdoutCopy) {
		task.AddOutputLine(
			fmt.Sprintf("[fo] Error reading stdout: %v", errStdoutCopy),
			design.TypeError, errCtx)
	}
	if errStderrCopy != nil && !isIgnorableErr(errStderrCopy) {
		task.AddOutputLine(
			fmt.Sprintf("[fo] Error reading stderr: %v", errStderrCopy),
			design.TypeError, errCtx)
	}

	exitCode := getExitCode(runErr, c.cfg.Debug)

	task.UpdateTaskContext()

	// Note: Classification already happened line-by-line during capture above
	// No need to re-scan and classify here
	if c.cfg.Debug {
		task.OutputLinesLock()
		lineCount := len(task.OutputLines)
		task.OutputLinesUnlock()
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Processed %d lines with streaming classification\n", lineCount)
	}

	return exitCode, runErr
}

func getExitCode(err error, debug bool) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code, ok := getExitCodeFromError(exitErr); ok {
			return code
		}
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] ExitError.Sys() not WaitStatus: %T\n", exitErr.Sys())
		}
		return 1
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Non-ExitError type: %T, error: %v\n", err, err)
	}

	if isCommandNotFoundError(err) {
		return 127
	}
	return 1
}

// isCommandNotFoundError checks if the error indicates the command was not found.
// This handles the standard exec.ErrNotFound and platform-specific string fallbacks
// for older Go versions or edge cases.
func isCommandNotFoundError(err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	// Fallback string matching for edge cases
	errStr := err.Error()
	if strings.Contains(errStr, "executable file not found") {
		return true
	}
	if runtime.GOOS != "windows" && strings.Contains(errStr, "no such file or directory") {
		return true
	}
	return false
}

func normalizeConfig(cfg ConsoleConfig) ConsoleConfig {
	normalized := cfg
	if normalized.ShowOutputMode == "" {
		normalized.ShowOutputMode = "on-fail"
	}
	if normalized.MaxBufferSize == 0 {
		normalized.MaxBufferSize = 10 * 1024 * 1024
	}
	if normalized.MaxLineLength == 0 {
		normalized.MaxLineLength = 1 * 1024 * 1024
	}
	if cfg.ShowTimerSet {
		normalized.ShowTimer = cfg.ShowTimer
	} else {
		normalized.ShowTimer = true
	}
	switch {
	case cfg.InlineSet:
		normalized.InlineProgress = cfg.InlineProgress
	case cfg.Design != nil:
		normalized.InlineProgress = cfg.Design.Style.UseInlineProgress
	default:
		normalized.InlineProgress = true
	}
	if normalized.Out == nil {
		normalized.Out = os.Stdout
	}
	if normalized.Err == nil {
		normalized.Err = os.Stderr
	}
	return normalized
}

func resolveDesignConfig(cfg ConsoleConfig) *design.Config {
	if cfg.Design != nil {
		return design.DeepCopyConfig(cfg.Design)
	}

	var base *design.Config
	switch cfg.ThemeName {
	case "ascii_minimal":
		base = design.ASCIIMinimalTheme()
	default:
		base = design.UnicodeVibrantTheme()
	}

	if cfg.Monochrome {
		design.ApplyMonochromeDefaults(base)
	}

	if cfg.UseBoxesSet {
		base.Style.UseBoxes = cfg.UseBoxes
	}
	base.Style.UseInlineProgress = cfg.InlineProgress
	if cfg.ShowTimerSet {
		base.Style.NoTimer = !cfg.ShowTimer
	}

	return base
}

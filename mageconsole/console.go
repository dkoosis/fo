package mageconsole

import (
	"bufio"
	"bytes"
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

func (c *Console) Run(label, command string, args ...string) (*TaskResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, getInterruptSignals()...)
	defer signal.Stop(sigChan)

	return c.runContext(ctx, cancel, sigChan, label, command, args)
}

var errCommandExited = errors.New("command exited with non-zero code")

func (c *Console) RunSimple(command string, args ...string) error {
	res, err := c.Run("", command, args...)
	if err != nil {
		return err
	}
	if res != nil && res.ExitCode != 0 {
		return fmt.Errorf("%w: %d", errCommandExited, res.ExitCode)
	}
	return nil
}

func (c *Console) runContext(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal, label, command string, args []string) (*TaskResult, error) {
	labelToUse := label
	if labelToUse == "" {
		labelToUse = filepath.Base(command)
	}

	designCfg := design.DeepCopyConfig(c.designConf)

	patternMatcher := design.NewPatternMatcher(designCfg)
	intent := patternMatcher.DetectCommandIntent(command, args)
	task := design.NewTask(labelToUse, intent, command, args, designCfg)

	useInlineProgress := designCfg.Style.UseInlineProgress && c.cfg.InlineProgress && !c.cfg.Stream

	progress := design.NewInlineProgress(task, c.cfg.Debug)

	if useInlineProgress {
		enableSpinner := !designCfg.Style.NoSpinner
		progress.Start(ctx, enableSpinner)
	} else {
		_, _ = c.cfg.Out.Write([]byte(task.RenderStartLine() + "\n"))
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = os.Environ()
	setProcessGroup(cmd)

	cmdDone := make(chan struct{})
	go func() {
		defer close(cmdDone)
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
			return
		case <-ctx.Done():
			if c.cfg.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Context done, ensuring process is killed if running.")
			}
			if cmd.Process != nil && cmd.ProcessState == nil {
				_ = killProcessGroupWithSIGKILL(cmd)
			}
			return
		case <-cmdDone:
			if c.cfg.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received, command finished naturally.")
			}
			return
		}
	}()

	var exitCode int
	var cmdRunError error
	var isActualFoStartupFailure bool

	if c.cfg.Stream {
		exitCode, cmdRunError = c.executeStreamMode(cmd, task)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	} else {
		exitCode, cmdRunError = c.executeCaptureMode(cmd, task, patternMatcher)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	}

	task.Complete(exitCode)

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG executeCommand] CI=%t, exitCode=%d, task.Status=%s, isActualFoStartupFailure=%t\n", c.cfg.Monochrome && !c.cfg.ShowTimer, exitCode, task.Status, isActualFoStartupFailure)
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
			if l.Type != design.TypeError || (!strings.HasPrefix(l.Content, "Error starting command") && !strings.HasPrefix(l.Content, "Error creating stdout pipe") && !strings.HasPrefix(l.Content, "Error creating stderr pipe") && !strings.HasPrefix(l.Content, "[fo] ")) {
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

func (c *Console) executeStreamMode(cmd *exec.Cmd, task *design.Task) (int, error) {
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode] Error creating stderr pipe, fallback to direct os.Stderr:", err)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr := cmd.Run()
		task.AddOutputLine(fmt.Sprintf("[fo] Error setting up stderr pipe for stream mode: %v", err), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
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
			if !errors.Is(scanErr, io.EOF) && !strings.Contains(scanErr.Error(), "file already closed") && !strings.Contains(scanErr.Error(), "broken pipe") {
				task.AddOutputLine(fmt.Sprintf("[fo] Error reading stderr in stream mode: %v", scanErr), design.TypeError, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 3})
			}
		} else if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner finished without error.")
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), startErr)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(c.cfg.Err, errMsg)

		_ = stderrPipe.Close()
		waitGroup.Wait()

		return getExitCode(startErr, c.cfg.Debug), startErr
	}

	runErr := cmd.Wait()
	waitGroup.Wait()

	exitCode := getExitCode(runErr, c.cfg.Debug)
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return exitCode, runErr
		}
	}

	return exitCode, runErr
}

func (c *Console) executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher) (int, error) {
	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Starting in CAPTURE mode\n")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stdout pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(c.cfg.Err, errMsg)
		return 1, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("[fo] Error creating stderr pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(c.cfg.Err, errMsg)
		_ = stdoutPipe.Close()
		return 1, err
	}

	var stdoutBuffer, stderrBuffer bytes.Buffer
	var wgRead sync.WaitGroup
	wgRead.Add(2)

	var errStdoutCopy, errStderrCopy error

	go func() {
		defer wgRead.Done()
		if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Copying stdoutPipe")
		}
		_, errStdoutCopy = io.Copy(&stdoutBuffer, stdoutPipe)
		if errStdoutCopy != nil && !errors.Is(errStdoutCopy, io.EOF) && !strings.Contains(errStdoutCopy.Error(), "file already closed") && !strings.Contains(errStdoutCopy.Error(), "broken pipe") {
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Error copying stdout: %v\n", errStdoutCopy)
			}
		} else if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Finished copying stdoutPipe (len: %d)\n", stdoutBuffer.Len())
		}
	}()

	go func() {
		defer wgRead.Done()
		if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Copying stderrPipe")
		}
		_, errStderrCopy = io.Copy(&stderrBuffer, stderrPipe)
		if errStderrCopy != nil && !errors.Is(errStderrCopy, io.EOF) && !strings.Contains(errStderrCopy.Error(), "file already closed") && !strings.Contains(errStderrCopy.Error(), "broken pipe") {
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Error copying stderr: %v\n", errStderrCopy)
			}
		} else if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Goroutine: Finished copying stderrPipe (len: %d)\n", stderrBuffer.Len())
		}
	}()

	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
		fmt.Fprintln(c.cfg.Err, errMsg)
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		wgRead.Wait()
		return getExitCode(err, c.cfg.Debug), err
	}

	var outputBuffer bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	go func() {
		defer close(stdoutDone)
		limitedReader := io.LimitReader(&stdoutBuffer, c.cfg.MaxBufferSize)
		_, _ = outputBuffer.ReadFrom(limitedReader)
	}()

	go func() {
		defer close(stderrDone)
		limitedReader := io.LimitReader(&stderrBuffer, c.cfg.MaxBufferSize)
		_, _ = outputBuffer.ReadFrom(limitedReader)
	}()

	runErr := cmd.Wait()
	wgRead.Wait()
	<-stdoutDone
	<-stderrDone

	if errStdoutCopy != nil && !errors.Is(errStdoutCopy, io.EOF) && !strings.Contains(errStdoutCopy.Error(), "file already closed") && !strings.Contains(errStdoutCopy.Error(), "broken pipe") {
		task.AddOutputLine(fmt.Sprintf("[fo] Error copying stdout: %v", errStdoutCopy), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
	}
	if errStderrCopy != nil && !errors.Is(errStderrCopy, io.EOF) && !strings.Contains(errStderrCopy.Error(), "file already closed") && !strings.Contains(errStderrCopy.Error(), "broken pipe") {
		task.AddOutputLine(fmt.Sprintf("[fo] Error copying stderr: %v", errStderrCopy), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5})
	}

	exitCode := getExitCode(runErr, c.cfg.Debug)

	task.UpdateTaskContext()

	output := outputBuffer.String()
	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Combined output size: %d bytes\n", len(output))
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	buf := make([]byte, 0, bufio.MaxScanTokenSize)
	scanner.Buffer(buf, c.cfg.MaxLineLength)

	lineIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
		if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Line %d classified as %s: %s\n", lineIndex, lineType, line)
		}
		task.AddOutputLine(line, lineType, lineContext)
		lineIndex++
	}

	if scanErr := scanner.Err(); scanErr != nil {
		if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Scanner error: %v\n", scanErr)
		}
		if !errors.Is(scanErr, bufio.ErrTooLong) && !errors.Is(scanErr, io.EOF) {
			task.AddOutputLine(fmt.Sprintf("[fo] Error scanning captured output: %v", scanErr), design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4})
		}
	} else if c.cfg.Debug {
		fmt.Fprintln(os.Stderr, "[DEBUG executeCaptureMode] Scanner finished without error.")
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

	if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found") || (runtime.GOOS != "windows" && strings.Contains(err.Error(), "no such file or directory")) {
		return 127
	}
	return 1
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

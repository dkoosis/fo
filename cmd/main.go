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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/davidkoosis/fo/cmd/internal/config"
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

func main() {
	flagConfig := parseFlags()

	if versionFlag {
		fmt.Printf("fo version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	fileConfig := config.LoadConfig()
	mergedConfig := config.MergeWithFlags(fileConfig, flagConfig)

	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1)
	}

	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(mergedConfig, cmdArgs[0])
	}

	if mergedConfig.Label == "" {
		mergedConfig.Label = cmdArgs[0]
	}

	localAppConfig := convertToInternalConfig(mergedConfig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	exitCode := executeCommand(ctx, cancel, sigChan, localAppConfig, cmdArgs)

	if localAppConfig.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d). Final Merged Config: %+v\n", exitCode, mergedConfig)
		fmt.Fprintf(os.Stderr, "[DEBUG main()] Local App Config: %+v\n", localAppConfig)
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

func parseFlags() *config.Config {
	cfg := &config.Config{}

	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&cfg.Debug, "d", false, "Enable debug output (shorthand).")
	flag.StringVar(&cfg.Label, "l", "", "Label for the task.")
	flag.StringVar(&cfg.Label, "label", "", "Label for the task (shorthand: -l).")
	flag.BoolVar(&cfg.Stream, "s", false, "Stream mode - print command's stdout/stderr live.")
	flag.BoolVar(&cfg.Stream, "stream", false, "Stream mode - print command's stdout/stderr live (shorthand: -s).")
	flag.StringVar(&cfg.ShowOutput, "show-output", "", "When to show captured output: on-fail, always, never. (Overrides file config)")
	flag.BoolVar(&cfg.NoTimer, "no-timer", false, "Disable showing the duration.")
	flag.BoolVar(&cfg.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flag.BoolVar(&cfg.CI, "ci", false, "Enable CI-friendly, plain-text output (implies --no-color, --no-timer).")

	var maxBufferSizeMB int
	var maxLineLengthKB int
	flag.IntVar(&maxBufferSizeMB, "max-buffer-size", 0,
		fmt.Sprintf("Maximum total buffer size in MB (per stream). Default from config: %dMB", config.DefaultMaxBufferSize/(1024*1024)))
	flag.IntVar(&maxLineLengthKB, "max-line-length", 0,
		fmt.Sprintf("Maximum length in KB for a single line. Default from config: %dKB", config.DefaultMaxLineLength/1024))

	flag.Parse()

	if cfg.CI {
		cfg.NoColor = true
		cfg.NoTimer = true
	}

	if val := cfg.ShowOutput; val != "" && !validShowOutputValues[val] {
		fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\n", val)
		fmt.Fprintln(os.Stderr, "Valid values are: on-fail, always, never")
		os.Exit(1)
	}

	if maxBufferSizeMB > 0 {
		cfg.MaxBufferSize = int64(maxBufferSizeMB) * 1024 * 1024
	}
	if maxLineLengthKB > 0 {
		cfg.MaxLineLength = maxLineLengthKB * 1024
	}
	return cfg
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

func executeCommand(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal, appConfig Config, cmdArgs []string) int {
	var designConfig *design.Config
	if appConfig.NoColor || appConfig.CI {
		designConfig = design.NoColorConfig()
		designConfig.Style.NoTimer = true // Ensure timer is off for CI/NoColor in design
	} else {
		designConfig = design.DefaultConfig()
		designConfig.Style.NoTimer = appConfig.NoTimer // Respect NoTimer flag from appConfig
	}

	patternMatcher := design.NewPatternMatcher(designConfig)
	intent := patternMatcher.DetectCommandIntent(cmdArgs[0], cmdArgs[1:])
	label := appConfig.Label
	if label == "" {
		label = intent
	}

	task := design.NewTask(label, intent, cmdArgs[0], cmdArgs[1:], designConfig)
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

	var exitCode int
	if appConfig.Stream {
		exitCode = executeStreamMode(cmd)
	} else {
		exitCode = executeCaptureMode(cmd, task, patternMatcher, appConfig)
	}

	task.Complete(exitCode) // This must be called before RenderEndLine

	// Print output for capture mode if necessary
	if !appConfig.Stream {
		// Condition for printing captured output
		shouldPrintCapturedOutput := (appConfig.ShowOutput == "on-fail" && exitCode != 0) || appConfig.ShowOutput == "always"

		if shouldPrintCapturedOutput {
			summary := task.RenderSummary()
			if summary != "" {
				fmt.Print(summary)
			}
			for _, line := range task.OutputLines {
				// This logic is to avoid re-printing [fo] messages if they were already shown live by processOutputPipe
				// when appConfig.ShowOutput == "always" during a buffer/line limit event.
				isFoBufferMessage := strings.Contains(line.Content, "[fo] BUFFER LIMIT")
				isFoLineMessage := strings.Contains(line.Content, "[fo] LINE LIMIT")

				// Apply De Morgan's Law: !(A || B) is equivalent to !A && !B
				// So, if it's NOT a buffer message AND NOT a line message, OR if we always show output, then print.
				if (!isFoBufferMessage && !isFoLineMessage) || appConfig.ShowOutput == "always" {
					fmt.Println(task.RenderOutputLine(line))
				}
			}
		}
	}
	fmt.Println(task.RenderEndLine())

	return exitCode
}

func executeStreamMode(cmd *exec.Cmd) int {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return getExitCode(err)
}

func executeCaptureMode(cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, appConfig Config) int {
	var wg sync.WaitGroup
	var bufferExceeded sync.Once

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating stdout pipe: %v", err)
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5}
		task.AddOutputLine(errMsg, design.TypeError, errCtx)
		fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
		return 1
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating stderr pipe: %v", err)
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5}
		task.AddOutputLine(errMsg, design.TypeError, errCtx)
		fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
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
			if source == "stderr" && lineType == design.TypeDetail {
				lineType = design.TypeError
				lineContext.Importance = 4
				lineContext.CognitiveLoad = design.LoadHigh
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
				// Applied De Morgan's Law for QF1001 and to satisfy SA9003 (no empty final else)
			} else if !strings.Contains(errScan.Error(), "file already closed") && !strings.Contains(errScan.Error(), "broken pipe") {
				// This 'else if' handles errors that are NOT ErrTooLong, "file already closed", or "broken pipe".
				errMsg := fmt.Sprintf("Error reading %s: %v", source, errScan)
				errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 4}
				task.AddOutputLine(errMsg, design.TypeError, errCtx)
				if appConfig.ShowOutput == "always" {
					fmt.Println(task.RenderOutputLine(task.OutputLines[len(task.OutputLines)-1]))
				}
			}
			// "file already closed" and "broken pipe" errors are intentionally ignored here.
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

func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

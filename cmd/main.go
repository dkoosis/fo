package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io" // Added for io.ReadCloser in scanOutputPipe
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	// Assuming your config package is correctly located here
	"github.com/davidkoosis/fo/cmd/internal/config"
	// Assuming your version package is correctly located here
	"github.com/davidkoosis/fo/cmd/internal/version"
)

// Config holds the command-line options relevant to fo's execution logic.
// This is distinct from config.Config which is used for loading/merging.
type Config struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool
	NoColor       bool
	CI            bool
	Debug         bool  // New field for the debug flag
	MaxBufferSize int64 // Total buffer size (in bytes)
	MaxLineLength int   // Maximum line length (in bytes)
}

// ANSI color codes.
const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorBlue  = "\033[34m"
)

// Status icons.
const (
	iconStart   = "▶️"
	iconSuccess = "✅"
	iconFailure = "❌"
)

// Other constants
const (
	// DefaultMaxBufferSize is 10MB total per stream (stdout/stderr)
	DefaultMaxBufferSize int64 = 10 * 1024 * 1024
	// DefaultMaxLineLength is 1MB per line
	DefaultMaxLineLength int = 1 * 1024 * 1024
)

// Valid options for --show-output flag.
var validShowOutputValues = map[string]bool{
	"on-fail": true,
	"always":  true,
	"never":   true,
}

// TimestampedLine represents a line of output with its timestamp and source.
type TimestampedLine struct {
	Time      time.Time
	Source    string // "stdout" or "stderr"
	Content   string
	Truncated bool
}

// versionFlag is set if the --version or -v flag is passed.
// It's a package-level variable to be checked after flag parsing.
var versionFlag bool

func main() {
	// Parse flags (this will also set the global versionFlag if -version is used)
	flagConfig := parseFlags()

	// Handle --version flag immediately after parsing
	if versionFlag {
		fmt.Printf("fo version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuildDate)
		os.Exit(0)
	}

	// Load config file
	fileConfig := config.LoadConfig()

	// Merge configurations (flags take precedence)
	// The MergeWithFlags function in your config package should be aware of the new Debug field
	// if you want it to be settable via .fo.yaml (though typically debug is CLI only).
	// For now, we assume flagConfig.Debug will correctly pass the CLI value.
	mergedConfig := config.MergeWithFlags(fileConfig, flagConfig)

	// Find the command to execute (after --).
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1) // Exit with error code 1 for bad arguments
	}

	// Apply command-specific preset
	if len(cmdArgs) > 0 {
		config.ApplyCommandPreset(mergedConfig, cmdArgs[0])
	}

	// Set default label if not provided and not overridden by preset
	if mergedConfig.Label == "" {
		mergedConfig.Label = cmdArgs[0] // Use command name as default label.
	}

	// Convert to the local Config type used by executeCommand
	localAppConfig := convertToInternalConfig(mergedConfig)

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to receive signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Execute the command with the given config.
	exitCode := executeCommand(ctx, cancel, sigChan, localAppConfig, cmdArgs)

	// Conditionally print the debug information
	if localAppConfig.Debug {
		// Use mergedConfig here if you want to see the state of the config.Config struct
		// or localAppConfig to see the state of the main.Config struct.
		// Using mergedConfig as it was the one used for the original debug message.
		fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d). Final Merged Config: %+v\n", exitCode, mergedConfig)
		fmt.Fprintf(os.Stderr, "[DEBUG main()] Local App Config: %+v\n", localAppConfig)
	}

	// Exit with the same code as the wrapped command or fo's own error code.
	os.Exit(exitCode)
}

// convertToInternalConfig converts from the shared config.Config to the local Config struct.
func convertToInternalConfig(cfg *config.Config) Config {
	return Config{
		Label:         cfg.Label,
		Stream:        cfg.Stream,
		ShowOutput:    cfg.ShowOutput,
		NoTimer:       cfg.NoTimer,
		NoColor:       cfg.NoColor,
		CI:            cfg.CI,
		Debug:         cfg.Debug, // Transfer the Debug field
		MaxBufferSize: cfg.MaxBufferSize,
		MaxLineLength: cfg.MaxLineLength,
	}
}

// parseFlags defines and parses command-line flags.
// It returns a config.Config struct populated with values from flags.
func parseFlags() *config.Config {
	// cfg stores values from command-line flags.
	cfg := &config.Config{} // Note: This is config.Config from your internal package

	// Define the --version flag (sets the global versionFlag)
	flag.BoolVar(&versionFlag, "version", false, "Print fo version and exit.")
	flag.BoolVar(&versionFlag, "v", false, "Print fo version and exit (shorthand).")

	// Define the --debug flag
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug output.")
	flag.BoolVar(&cfg.Debug, "d", false, "Enable debug output (shorthand).")

	// Define other flags
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
		fmt.Sprintf("Maximum total buffer size in MB (per stream). Default from config: %dMB", config.DefaultMaxBufferSize/1024/1024))

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

// findCommandArgs extracts the command and its arguments that appear after "--".
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
	printStartLine(appConfig)
	startTime := time.Now()

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
			go func() {
				select {
				case <-cmdDone:
				case <-time.After(2 * time.Second):
					cancel()
				}
			}()
		case <-ctx.Done():
			if cmd.Process != nil && cmd.ProcessState == nil {
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				} else {
					_ = cmd.Process.Kill()
				}
			}
		}
	}()

	var exitCode int
	if appConfig.Stream {
		exitCode = executeStreamMode(cmd, appConfig, startTime)
	} else {
		exitCode = executeCaptureMode(cmd, appConfig, startTime, cmdArgs)
	}
	return exitCode
}

func executeStreamMode(cmd *exec.Cmd, appConfig Config, startTime time.Time) int {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	duration := time.Since(startTime)
	exitCode := getExitCode(err)
	printEndLine(appConfig, exitCode, duration)
	return exitCode
}

func executeCaptureMode(cmd *exec.Cmd, appConfig Config, startTime time.Time, cmdArgs []string) int {
	var wg sync.WaitGroup
	var bufferExceeded sync.Once

	stdoutLinesChan := make(chan TimestampedLine, 200)
	stderrLinesChan := make(chan TimestampedLine, 200)
	allCapturedLines := make([]TimestampedLine, 0, 400)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		printEndLine(appConfig, 1, time.Since(startTime))
		return 1
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error creating stderr pipe: %v\n", err)
		printEndLine(appConfig, 1, time.Since(startTime))
		return 1
	}

	wg.Add(1)
	go scanOutputPipe(stdoutPipe, "stdout", stdoutLinesChan, appConfig, &bufferExceeded, &wg)
	wg.Add(1)
	go scanOutputPipe(stderrPipe, "stderr", stderrLinesChan, appConfig, &bufferExceeded, &wg)

	collectionDone := make(chan struct{})
	go func() {
		defer close(collectionDone)
		activeStdoutChan := stdoutLinesChan
		activeStderrChan := stderrLinesChan
		for activeStdoutChan != nil || activeStderrChan != nil {
			select {
			case line, ok := <-activeStdoutChan:
				if !ok {
					activeStdoutChan = nil
					continue
				}
				allCapturedLines = append(allCapturedLines, line)
			case line, ok := <-activeStderrChan:
				if !ok {
					activeStderrChan = nil
					continue
				}
				allCapturedLines = append(allCapturedLines, line)
			}
		}
	}()

	startErr := cmd.Start()
	var cmdWaitErr error
	if startErr != nil {
		if len(cmdArgs) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Error starting command '%s': %v\n", cmdArgs[0], startErr)
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "Error starting command (name unavailable): %v\n", startErr)
		}
	} else {
		cmdWaitErr = cmd.Wait()
	}

	wg.Wait()
	<-collectionDone

	duration := time.Since(startTime)
	var finalErr error
	if startErr != nil {
		finalErr = startErr
	} else {
		finalErr = cmdWaitErr
	}
	exitCode := getExitCode(finalErr)
	printEndLine(appConfig, exitCode, duration)

	sort.Slice(allCapturedLines, func(i, j int) bool {
		return allCapturedLines[i].Time.Before(allCapturedLines[j].Time)
	})

	showOutput := false
	switch appConfig.ShowOutput {
	case "always":
		showOutput = true
	case "on-fail":
		showOutput = (exitCode != 0)
	}

	if showOutput && len(allCapturedLines) > 0 {
		_, _ = fmt.Println("--- Captured output: ---")
		for _, line := range allCapturedLines {
			if line.Truncated {
				if appConfig.NoColor || appConfig.CI {
					_, _ = fmt.Printf("%s\n", line.Content)
				} else {
					_, _ = fmt.Printf("%s%s%s\n", colorRed, line.Content, colorReset)
				}
			} else {
				_, _ = fmt.Println(line.Content)
			}
		}
		_, _ = fmt.Println()
	}
	return exitCode
}

func scanOutputPipe(pipe io.ReadCloser, source string, outChan chan<- TimestampedLine, appConfig Config, bufferExceeded *sync.Once, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(outChan)

	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), appConfig.MaxLineLength)

	var totalBytes int64
	for scanner.Scan() {
		line := scanner.Text()
		lineSize := int64(len(line))
		if totalBytes+lineSize > appConfig.MaxBufferSize {
			bufferExceeded.Do(func() {
				exceededMsg := fmt.Sprintf("[fo] ERROR: Total %s buffer size limit (%d MB) exceeded. Further output truncated.", source, appConfig.MaxBufferSize/1024/1024)
				outChan <- TimestampedLine{Time: time.Now(), Source: source, Content: exceededMsg, Truncated: true}
				_, _ = fmt.Fprintf(os.Stderr, "%s\n", exceededMsg)
			})
			break
		}
		totalBytes += lineSize
		outChan <- TimestampedLine{Time: time.Now(), Source: source, Content: line}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			bufferExceeded.Do(func() {
				exceededMsg := fmt.Sprintf("[fo] ERROR: Maximum line length (%d KB) exceeded in %s. Line truncated.", appConfig.MaxLineLength/1024, source)
				outChan <- TimestampedLine{Time: time.Now(), Source: source, Content: exceededMsg, Truncated: true}
				_, _ = fmt.Fprintf(os.Stderr, "%s\n", exceededMsg)
			})
		} else if !strings.Contains(err.Error(), "file already closed") && !strings.Contains(err.Error(), "broken pipe") {
			fmt.Fprintf(os.Stderr, "[fo] Error reading %s: %v\n", source, err)
		}
	}
}

func printStartLine(appConfig Config) {
	label := appConfig.Label
	icon := iconStart
	color := colorBlue
	if appConfig.CI || appConfig.NoColor {
		fmt.Printf("[START] %s...\n", label)
	} else {
		fmt.Printf("%s %s%s...%s\n", icon, color, label, colorReset)
	}
}

func printEndLine(appConfig Config, exitCode int, duration time.Duration) {
	label := appConfig.Label
	var icon string
	var color string
	if exitCode == 0 {
		icon = iconSuccess
		color = colorGreen
	} else {
		icon = iconFailure
		color = colorRed
	}
	durationStr := ""
	if !appConfig.CI && !appConfig.NoTimer {
		durationStr = fmt.Sprintf(" (%s)", formatDuration(duration))
	}
	if appConfig.CI || appConfig.NoColor {
		statusText := "[SUCCESS]"
		if exitCode != 0 {
			statusText = "[FAILED]"
		}
		fmt.Printf("%s %s%s\n", statusText, label, durationStr)
	} else {
		fmt.Printf("%s %s%s%s%s\n", icon, color, label, durationStr, colorReset)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		secondsFraction := d.Seconds() - float64(minutes*60)
		return fmt.Sprintf("%dm%.1fs", minutes, secondsFraction)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
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

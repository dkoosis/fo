package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Config holds the command-line options.
type Config struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool
	NoColor       bool
	CI            bool
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

func main() {
	config := parseFlags()

	// Find the command to execute (after --).
	cmdArgs := findCommandArgs()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: fo [flags] -- <COMMAND> [ARGS...]")
		os.Exit(1)
	}

	// Set default label if not provided.
	if config.Label == "" {
		config.Label = cmdArgs[0] // Use command name as default label.
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to receive signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Execute the command with the given config.
	exitCode := executeCommand(ctx, cancel, sigChan, config, cmdArgs)

	fmt.Fprintf(os.Stderr, "[DEBUG main()] about to os.Exit(%d). Config: %+v\n", exitCode, config)

	// Exit with the same code as the wrapped command.
	os.Exit(exitCode)
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.Label, "l", "", "Label for the task.")
	flag.StringVar(&config.Label, "label", "", "Label for the task (shorthand: -l).")

	flag.BoolVar(&config.Stream, "s", false, "Stream mode - print command's stdout/stderr live.")
	flag.BoolVar(&config.Stream, "stream", false, "Stream mode - print command's stdout/stderr live (shorthand: -s).")

	flag.StringVar(&config.ShowOutput, "show-output", "on-fail", "When to show captured output: on-fail (default), always, never.")

	flag.BoolVar(&config.NoTimer, "no-timer", false, "Disable showing the duration.")
	flag.BoolVar(&config.NoColor, "no-color", false, "Disable ANSI color/styling output.")
	flag.BoolVar(&config.CI, "ci", false, "Enable CI-friendly, plain-text output (implies --no-color, --no-timer).")

	maxBufferSizeMB := flag.Int("max-buffer-size", int(DefaultMaxBufferSize/1024/1024),
		"Maximum total buffer size in MB (per stream) for capturing command output. Default: 10MB")

	maxLineLengthKB := flag.Int("max-line-length", int(DefaultMaxLineLength/1024),
		"Maximum length in KB for a single line of output. Default: 1024KB (1MB)")

	// Parse flags but stop at --.
	flag.Parse()

	// Apply implications of --ci flag.
	if config.CI {
		config.NoColor = true
		config.NoTimer = true
	}

	// Validate ShowOutput value.
	if !validShowOutputValues[config.ShowOutput] {
		fmt.Fprintf(os.Stderr, "Error: Invalid value for --show-output: %s\n", config.ShowOutput)
		fmt.Fprintln(os.Stderr, "Valid values are: on-fail, always, never")
		os.Exit(1)
	}

	// Convert MB to bytes for buffer size
	config.MaxBufferSize = int64(*maxBufferSizeMB) * 1024 * 1024

	// Convert KB to bytes for line length
	config.MaxLineLength = *maxLineLengthKB * 1024

	return config
}

func findCommandArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i < len(os.Args)-1 {
			return os.Args[i+1:]
		}
	}
	return nil
}

func executeCommand(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal, config Config, cmdArgs []string) int {
	// Print start line.
	printStartLine(config)

	startTime := time.Now()

	// Create command using the provided context
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = os.Environ() // Inherit environment.

	// Set process group for better signal handling
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create a new process group
	}

	// Handle signals in a goroutine
	cmdDone := make(chan struct{})
	go func() {
		select {
		case sig := <-sigChan:
			fmt.Fprintf(os.Stderr, "[DEBUG signal handler] Received signal: %v\n", sig)

			// Try to forward the signal to the process group
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				_, _ = fmt.Fprintf(os.Stderr, "[DEBUG signal handler] Forwarding signal to process group %d\n", pgid)
				// Send the signal to the process group
				if err := syscall.Kill(-pgid, sig.(syscall.Signal)); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[DEBUG signal handler] Error sending signal to process group: %v\n", err)
				}
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "[DEBUG signal handler] Failed to get pgid, sending signal directly: %v\n", err)
				// Fall back to sending to just the process
				if err := cmd.Process.Signal(sig); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[DEBUG signal handler] Error sending signal directly to process: %v\n", err)
				}
			}

			// Cancel our context after a brief delay if the command doesn't exit
			go func() {
				select {
				case <-cmdDone:
					// Command exited, no need to cancel
					return
				case <-time.After(2 * time.Second):
					// Command didn't exit after signal, cancel the context
					_, _ = fmt.Fprintf(os.Stderr, "Warning: Command did not exit after signal, forcibly terminating\n")
					cancel()
				}
			}()

		case <-cmdDone:
			// Command is done, stop handling signals
			return
		}
	}()

	var exitCode int
	if config.Stream {
		// STREAM MODE.
		exitCode = executeStreamMode(cmd, config, startTime)
	} else {
		// CAPTURE MODE.
		exitCode = executeCaptureMode(cmd, config, startTime)
	}

	// Signal that command is done
	close(cmdDone)

	return exitCode
}

func executeStreamMode(cmd *exec.Cmd, config Config, startTime time.Time) int {
	// Set up direct streaming of output.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command.
	err := cmd.Run()

	// Print end status.
	duration := time.Since(startTime)
	exitCode := getExitCode(err)
	printEndLine(config, exitCode, duration)

	return exitCode
}

func executeCaptureMode(cmd *exec.Cmd, config Config, startTime time.Time) int {
	var wg sync.WaitGroup
	var bufferExceeded sync.Once

	// Create channels for line output with timestamps
	stdoutLines := make(chan TimestampedLine, 100)
	stderrLines := make(chan TimestampedLine, 100)
	allLines := make([]TimestampedLine, 0, 200)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		return 1
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error creating stderr pipe: %v\n", err)
		return 1
	}

	// Process stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stdoutLines)

		scanner := bufio.NewScanner(stdoutPipe)
		// Use config.MaxLineLength for the scanner's buffer, not MaxBufferSize
		scanner.Buffer(make([]byte, 1024*64), config.MaxLineLength)

		var totalBytes int64
		for scanner.Scan() {
			line := scanner.Text()
			lineSize := int64(len(line))
			totalBytes += lineSize

			// Check for total buffer size limit
			if totalBytes > config.MaxBufferSize {
				// Buffer limit reached
				bufferExceeded.Do(func() {
					exceededMsg := fmt.Sprintf("[fo] ERROR: Total stdout buffer size limit (%d MB) exceeded. Further output truncated.", config.MaxBufferSize/1024/1024)
					stdoutLines <- TimestampedLine{
						Time:      time.Now(),
						Source:    "stdout",
						Content:   exceededMsg,
						Truncated: true,
					}
					_, _ = fmt.Fprintf(os.Stderr, "%s\n", exceededMsg)
				})
				break // Stop reading more lines
			}

			stdoutLines <- TimestampedLine{
				Time:    time.Now(),
				Source:  "stdout",
				Content: line,
			}
		}

		// Check if scanners are still valid
		if scanner.Err() != nil {
			err := scanner.Err()
			if strings.Contains(err.Error(), "token too long") || strings.Contains(err.Error(), "buffer size exceeded") {
				bufferExceeded.Do(func() {
					exceededMsg := fmt.Sprintf("[fo] ERROR: Maximum line length (%d KB) exceeded in stdout. Line truncated.", config.MaxLineLength/1024)
					stdoutLines <- TimestampedLine{
						Time:      time.Now(),
						Source:    "stdout",
						Content:   exceededMsg,
						Truncated: true,
					}
					_, _ = fmt.Fprintf(os.Stderr, "%s\n", exceededMsg)
				})
			} else if !strings.Contains(err.Error(), "file already closed") && !strings.Contains(err.Error(), "broken pipe") {
				_, _ = fmt.Fprintf(os.Stderr, "Error reading stdout: %v\n", err)
			}
		}
	}()

	// Process stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stderrLines)

		scanner := bufio.NewScanner(stderrPipe)
		// Use config.MaxLineLength for the scanner's buffer, not MaxBufferSize
		scanner.Buffer(make([]byte, 1024*64), config.MaxLineLength)

		var totalBytes int64
		for scanner.Scan() {
			line := scanner.Text()
			lineSize := int64(len(line))
			totalBytes += lineSize

			// Check for total buffer size limit
			if totalBytes > config.MaxBufferSize {
				// Buffer limit reached
				bufferExceeded.Do(func() {
					exceededMsg := fmt.Sprintf("[fo] ERROR: Total stderr buffer size limit (%d MB) exceeded. Further output truncated.", config.MaxBufferSize/1024/1024)
					stderrLines <- TimestampedLine{
						Time:      time.Now(),
						Source:    "stderr",
						Content:   exceededMsg,
						Truncated: true,
					}
					_, _ = fmt.Fprintf(os.Stderr, "%s\n", exceededMsg)
				})
				break // Stop reading more lines
			}

			stderrLines <- TimestampedLine{
				Time:    time.Now(),
				Source:  "stderr",
				Content: line,
			}
		}

		// Check if scanners are still valid
		if scanner.Err() != nil {
			err := scanner.Err()
			if strings.Contains(err.Error(), "token too long") || strings.Contains(err.Error(), "buffer size exceeded") {
				bufferExceeded.Do(func() {
					exceededMsg := fmt.Sprintf("[fo] ERROR: Maximum line length (%d KB) exceeded in stderr. Line truncated.", config.MaxLineLength/1024)
					stderrLines <- TimestampedLine{
						Time:      time.Now(),
						Source:    "stderr",
						Content:   exceededMsg,
						Truncated: true,
					}
					_, _ = fmt.Fprintf(os.Stderr, "%s\n", exceededMsg)
				})
			} else if !strings.Contains(err.Error(), "file already closed") && !strings.Contains(err.Error(), "broken pipe") {
				_, _ = fmt.Fprintf(os.Stderr, "Error reading stderr: %v\n", err)
			}
		}
	}()

	// Collect all lines in a merged goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()

		stdoutOpen, stderrOpen := true, true
		for stdoutOpen || stderrOpen {
			select {
			case line, ok := <-stdoutLines:
				if !ok {
					stdoutOpen = false
					continue
				}
				allLines = append(allLines, line)
			case line, ok := <-stderrLines:
				if !ok {
					stderrOpen = false
					continue
				}
				allLines = append(allLines, line)
			}
		}
	}()

	// Start the command.
	startErr := cmd.Start()

	// Always wait for output collection to complete, regardless of cmd.Start() outcome
	var cmdWaitErr error
	if startErr == nil {
		// Only wait for command to complete if it started successfully
		cmdWaitErr = cmd.Wait()
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Error starting command: %v\n", startErr)
	}

	// Wait for output collection to complete.
	wg.Wait()

	// Get exit code and duration.
	duration := time.Since(startTime)

	// Use the appropriate error based on what happened
	exitCode := 1
	if startErr == nil {
		exitCode = getExitCode(cmdWaitErr)
	}

	// Print end status.
	printEndLine(config, exitCode, duration)

	// Sort lines by timestamp
	sort.Slice(allLines, func(i, j int) bool {
		return allLines[i].Time.Before(allLines[j].Time)
	})

	// Determine if we should show the captured output.
	showOutput := false
	switch config.ShowOutput {
	case "always":
		showOutput = true
	case "on-fail":
		showOutput = (exitCode != 0)
	case "never":
		showOutput = false
	}

	// Print captured output if needed.
	if showOutput && len(allLines) > 0 {
		_, _ = fmt.Println("--- Captured output: ---")

		for _, line := range allLines {
			// For truncated warning lines, print in a distinctive way
			if line.Truncated {
				_, _ = fmt.Printf("%s%s%s\n", colorRed, line.Content, colorReset)
			} else {
				_, _ = fmt.Println(line.Content)
			}
		}

		// End with a newline for better readability
		_, _ = fmt.Println()
	}

	return exitCode
}

func printStartLine(config Config) {
	if config.NoColor {
		fmt.Printf("%s %s...\n", getStartIcon(config), config.Label)
	} else {
		fmt.Printf("%s %s%s...%s\n", getStartIcon(config), colorBlue, config.Label, colorReset)
	}
}

func printEndLine(config Config, exitCode int, duration time.Duration) {
	var icon string
	var colorCode string

	if exitCode == 0 {
		icon = getSuccessIcon(config)
		colorCode = colorGreen
	} else {
		icon = getFailureIcon(config)
		colorCode = colorRed
	}

	durationStr := ""
	if !config.NoTimer {
		durationStr = fmt.Sprintf(" (%s)", formatDuration(duration))
	}

	if config.NoColor {
		fmt.Printf("%s %s%s\n", icon, config.Label, durationStr)
	} else {
		fmt.Printf("%s %s%s%s%s\n", icon, colorCode, config.Label, durationStr, colorReset)
	}
}

func getStartIcon(config Config) string {
	if config.CI || config.NoColor {
		return "[START]"
	}
	return iconStart
}

func getSuccessIcon(config Config) string {
	if config.CI || config.NoColor {
		return "[SUCCESS]"
	}
	return iconSuccess
}

func getFailureIcon(config Config) string {
	if config.CI || config.NoColor {
		return "[FAILED]"
	}
	return iconFailure
}

func formatDuration(d time.Duration) string {
	// Format duration in a human-readable way.
	if d.Hours() >= 1 {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if d.Minutes() >= 1 {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		ms := int(d.Milliseconds()) % 1000
		return fmt.Sprintf("%dm%d.%ds", m, s, ms/100) // Corrected to ms/100 for one decimal place.
	} else {
		s := d.Seconds()
		return fmt.Sprintf("%.1fs", s)
	}
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	// These are debug logs; if they fail, we can continue execution
	_, _ = fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Received err: <%v> (type: <%s>)\n", err, reflect.TypeOf(err))

	if exitErr, ok := err.(*exec.ExitError); ok {
		_, _ = fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Successfully asserted to *exec.ExitError, code: %d\n", exitErr.ExitCode())
		return exitErr.ExitCode()
	}

	_, _ = fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Failed to assert to *exec.ExitError. Falling back to generic error.\n")
	_, _ = fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
	return 1
}

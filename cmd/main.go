package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Config holds the command-line options.
type Config struct {
	Label      string
	Stream     bool
	ShowOutput string
	NoTimer    bool
	NoColor    bool
	CI         bool
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

// Valid options for --show-output flag.
var validShowOutputValues = map[string]bool{
	"on-fail": true,
	"always":  true,
	"never":   true,
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

	// Execute the command with the given config.
	exitCode := executeCommand(config, cmdArgs)

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

func executeCommand(config Config, cmdArgs []string) int {
	// Create context for the command (for future timeout support).
	ctx := context.Background()

	// Prepare command.
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = os.Environ() // Inherit environment.

	// Print start line.
	printStartLine(config)

	startTime := time.Now()

	if config.Stream {
		// STREAM MODE.
		return executeStreamMode(cmd, config, startTime)
	} else {
		// CAPTURE MODE.
		return executeCaptureMode(cmd, config, startTime)
	}
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
	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		return 1
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stderr pipe: %v\n", err)
		return 1
	}

	// Start copying output to buffers in goroutines.
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, copyErr := io.Copy(&stdout, stdoutPipe); copyErr != nil {
			fmt.Fprintf(os.Stderr, "Error copying stdout: %v\n", copyErr)
		}
	}()

	go func() {
		defer wg.Done()
		if _, copyErr := io.Copy(&stderr, stderrPipe); copyErr != nil {
			fmt.Fprintf(os.Stderr, "Error copying stderr: %v\n", copyErr)
		}
	}()

	// Start the command.
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting command: %v\n", err)
		duration := time.Since(startTime)
		printEndLine(config, 1, duration) // Mark as failed.
		return 1
	}

	// Wait for command to complete.
	err = cmd.Wait()

	// Wait for output collection to complete.
	wg.Wait()

	// Get exit code and duration.
	duration := time.Since(startTime)
	exitCode := getExitCode(err)

	// Print end status.
	printEndLine(config, exitCode, duration)

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
	if showOutput && (stdout.Len() > 0 || stderr.Len() > 0) {
		fmt.Println("--- Captured output: ---")
		if stdout.Len() > 0 {
			fmt.Print(stdout.String())
			// Ensure newline at the end.
			if !strings.HasSuffix(stdout.String(), "\n") {
				fmt.Println()
			}
		}
		if stderr.Len() > 0 {
			fmt.Print(stderr.String())
			// Ensure newline at the end.
			if !strings.HasSuffix(stderr.String(), "\n") {
				fmt.Println()
			}
		}
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
	fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Received err: <%v> (type: <%s>)\n", err, reflect.TypeOf(err))
	if exitErr, ok := err.(*exec.ExitError); ok {
		fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Successfully asserted to *exec.ExitError, code: %d\n", exitErr.ExitCode())
		return exitErr.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Failed to assert to *exec.ExitError. Falling back to generic error.\n")
	fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
	return 1
}

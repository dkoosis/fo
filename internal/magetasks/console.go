package magetasks

import (
	"fmt"
	"strings"
)

// PrintH1Header prints a top-level header with decoration.
func PrintH1Header(title string) {
	width := 80
	fmt.Println()
	fmt.Println(strings.Repeat("=", width))
	padding := (width - len(title)) / 2
	fmt.Printf("%s%s\n", strings.Repeat(" ", padding), title)
	fmt.Println(strings.Repeat("=", width))
	fmt.Println()
}

// PrintH2Header prints a section header.
func PrintH2Header(title string) {
	fmt.Println()
	fmt.Printf("=== %s ===\n", title)
	fmt.Println()
}

// PrintSuccess prints a success message.
func PrintSuccess(msg string) {
	fmt.Printf("✅ %s\n", msg)
}

// PrintWarning prints a warning message.
func PrintWarning(msg string) {
	fmt.Printf("⚠️  %s\n", msg)
}

// PrintError prints an error message.
func PrintError(msg string) {
	fmt.Printf("❌ %s\n", msg)
}

// PrintInfo prints an info message.
func PrintInfo(msg string) {
	fmt.Printf("ℹ️  %s\n", msg)
}
